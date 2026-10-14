package kv

import (
	"context"
	"errors"
	"testing"
	"time"

	"raftkv/internal/clock"
	"raftkv/internal/raft"
)

// localread: a leader must confirm it is still leader (ReadIndex: commit an
// entry of its term, then a heartbeat round) before serving a read, since a
// newer leader may have been elected behind a partition. Here node 1 wins its
// election and is then cut off: no AppendEntries reaches a follower. Its
// clock is frozen so it does not notice the lost quorum and step down. The
// buggy store answers the read from local state anyway.
//
//	go test ./internal/kv -run TestBugLocalRead                            # passes
//	RAFTKV_BUGS=localread go test -tags bugs ./internal/kv -run TestBugLocalRead  # fails
func TestBugLocalRead(t *testing.T) {
	clk := clock.NewSkewable()
	s, err := New(Config{
		Raft: raft.Config{
			ID:              1,
			Peers:           []uint64{1, 2, 3},
			ElectionTimeout: 200 * time.Millisecond,
			Clock:           clk,
			Storage:         raft.NewMemStorage(),
			Transport:       votesOnly{},
		},
		ReadMode: ReadIndex,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)

	deadline := time.Now().Add(10 * time.Second)
	for s.rf.Status().Role != raft.Leader {
		if time.Now().After(deadline) {
			t.Fatal("node 1 did not become leader")
		}
		time.Sleep(time.Millisecond)
	}
	clk.SetRate(0)
	if s.rf.Status().Role != raft.Leader {
		t.Skip("node 1 stepped down before its clock was frozen; rerun")
	}

	s.mu.Lock()
	s.data["k"] = "old"
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if r, err := s.Get(ctx, "k"); err == nil {
		t.Fatalf("a leader that cannot reach any follower served a read (%q) without confirming leadership", r.Value)
	}
}

// votesOnly grants every vote and loses every other message.
type votesOnly struct{}

var errCut = errors.New("test: cut off")

func (votesOnly) RequestVote(_ context.Context, _ uint64, req *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	return &raft.RequestVoteResponse{Term: req.Term, Granted: true}, nil
}
func (votesOnly) AppendEntries(context.Context, uint64, *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	return nil, errCut
}
func (votesOnly) InstallSnapshot(context.Context, uint64, *raft.InstallSnapshotRequest) (*raft.InstallSnapshotResponse, error) {
	return nil, errCut
}
