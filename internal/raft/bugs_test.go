package raft

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Deterministic tests for the injected Raft bugs (RQ3). Each drives one node
// directly, with no network and no timing, so it fails every time its bug is
// switched on and passes otherwise:
//
//	go test ./internal/raft -run TestBug                          # all pass
//	RAFTKV_BUGS=votenopersist go test -tags bugs ./internal/raft -run TestBugVoteNoPersist  # fails
//
// They answer "can this bug fire?" independently of the harness, so a
// "not detected" E6 cell can be read as a harness blind spot rather than an
// unreachable bug.

// downTransport fails every RPC: the node under test only hears the requests
// the test hands it.
type downTransport struct{}

var errDown = errors.New("test: network down")

func (downTransport) RequestVote(context.Context, uint64, *RequestVoteRequest) (*RequestVoteResponse, error) {
	return nil, errDown
}
func (downTransport) AppendEntries(context.Context, uint64, *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	return nil, errDown
}
func (downTransport) InstallSnapshot(context.Context, uint64, *InstallSnapshotRequest) (*InstallSnapshotResponse, error) {
	return nil, errDown
}

// startLone starts node 1 of a 5-node cluster on st. The election timeout is
// long enough that the node never times out during a test.
func startLone(t *testing.T, st *MemStorage) *Raft {
	t.Helper()
	apply := make(chan ApplyMsg, 64)
	r, err := New(Config{
		ID:              1,
		Peers:           []uint64{1, 2, 3, 4, 5},
		ElectionTimeout: time.Minute,
		Storage:         st,
		Transport:       downTransport{},
		Apply:           apply,
	})
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-apply:
			case <-stop:
				return
			}
		}
	}()
	t.Cleanup(func() { r.Stop(); close(stop) })
	return r
}

// preload writes a durable starting state, as if the node had run before.
func preload(t *testing.T, hs HardState, ents ...Entry) *MemStorage {
	t.Helper()
	st := NewMemStorage()
	if err := st.SetHardState(hs); err != nil {
		t.Fatal(err)
	}
	if err := st.Append(ents); err != nil {
		t.Fatal(err)
	}
	if err := st.Sync(); err != nil {
		t.Fatal(err)
	}
	return st
}

// votenopersist: a node that votes, crashes and restarts must not vote for a
// second candidate in the same term (Figure 2: persist votedFor before
// responding). Same-term requests skip the restart embargo in inLease, so the
// buggy node grants the second vote: two leaders become possible in one term.
func TestBugVoteNoPersist(t *testing.T) {
	st := preload(t, HardState{Term: 5})
	r := startLone(t, st)
	if resp := r.HandleRequestVote(&RequestVoteRequest{Term: 5, Candidate: 2}); !resp.Granted {
		t.Fatal("first vote in term 5 was not granted")
	}
	r.Stop()
	st.Crash()

	r = startLone(t, st)
	if resp := r.HandleRequestVote(&RequestVoteRequest{Term: 5, Candidate: 3}); resp.Granted {
		t.Fatal("after a crash and restart, the node voted twice in term 5 (for 2, then 3): votedFor was not persisted")
	}
}

// notruncate: when an AppendEntries conflicts with the follower's log, the
// follower must delete the conflicting entry and everything after it
// (Figure 2, AppendEntries rule 3). The buggy follower overwrites only the
// entries it was sent and keeps the stale suffix, breaking Log Matching.
func TestBugNoTruncate(t *testing.T) {
	st := preload(t, HardState{Term: 1},
		Entry{Index: 1, Term: 1}, Entry{Index: 2, Term: 1}, Entry{Index: 3, Term: 1})
	r := startLone(t, st)
	resp := r.HandleAppendEntries(&AppendEntriesRequest{
		Term: 2, Leader: 2, PrevIndex: 0, PrevTerm: 0,
		Entries: []Entry{{Index: 1, Term: 2}},
	})
	if !resp.Success {
		t.Fatalf("append rejected: %+v", resp)
	}
	if got := r.Status().LastIndex; got != 1 {
		t.Fatalf("log ends at %d after a conflicting append at index 1, want 1: stale entries from term 1 survived", got)
	}
}

// figure8: a leader may only commit an entry from an earlier term
// indirectly, by committing one from its own term (§5.4.2, Figure 8). Here
// the term-4 leader's index 2 (term 2) sits on a majority but nothing from
// term 4 does; counting replicas would commit it.
//
// This is white-box: it sets the leader's view directly instead of staging
// Figure 8's three leaderships. On a real election the no-op appended by
// becomeLeader usually travels with the old entries, which hides this bug.
func TestBugFigure8(t *testing.T) {
	st := preload(t, HardState{Term: 4, Vote: 1},
		Entry{Index: 1, Term: 1}, Entry{Index: 2, Term: 2})
	r := startLone(t, st)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.role = Leader
	r.leader = r.id
	r.commit = 1
	r.prs = map[uint64]*progress{
		2: {match: 2, next: 3}, 3: {match: 2, next: 3},
		4: {match: 0, next: 1}, 5: {match: 0, next: 1},
	}
	r.waitDurable(r.enqSeq)
	r.maybeCommit()
	if r.commit != 1 {
		t.Fatalf("term-4 leader committed index %d (term 2) by counting replicas, want commit 1", r.commit)
	}
}
