package raft

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Audit of RAFT-DEEP-DIVE §16.2: becomeFollower records a higher term with
// persistHardState() but does not wait for it. These tests pin down which
// responses can leave a node before the term (or vote) they carry is in
// stable storage, and check that the ones that promise something (a vote,
// a log match) cannot. They hold every storage write, so "durable" below
// means what the node would reload after a crash.

// stillClock is a clock that only moves when the test moves it, so the
// ticker never starts an election behind the test's back.
type stillClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *stillClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *stillClock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// heldStorage holds every write (hard state, entries, snapshots) until
// release, or forever once dropped, as if the process died first.
type heldStorage struct {
	*gatedStorage
}

func (h heldStorage) wait() error {
	g := h.gatedStorage
	g.mu.Lock()
	defer g.mu.Unlock()
	for g.closed && !g.dropped {
		g.cond.Wait()
	}
	if g.dropped {
		return errDropped
	}
	return nil
}

func (h heldStorage) SetHardState(hs HardState) error {
	if err := h.wait(); err != nil {
		return err
	}
	return h.MemStorage.SetHardState(hs)
}

func (h heldStorage) SaveSnapshot(s Snapshot, hs HardState, ents []Entry) error {
	if err := h.wait(); err != nil {
		return err
	}
	return h.MemStorage.SaveSnapshot(s, hs, ents)
}

const auditTimeout = time.Second

// startAudit starts node 1 of five on mem with all writes held, and moves its
// clock past the restart embargo (one election timeout) but not past its
// election deadline, so it answers votes and never campaigns.
func startAudit(t *testing.T, mem *MemStorage) (*Raft, heldStorage) {
	t.Helper()
	st := heldStorage{newGated(mem)}
	clk := &stillClock{t: time.Unix(1e9, 0)}
	apply := make(chan ApplyMsg, 64)
	r, err := New(Config{ID: 1, Peers: []uint64{1, 2, 3, 4, 5}, ElectionTimeout: auditTimeout,
		Clock: clk, Storage: st, Transport: downTransport{}, Apply: apply})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range apply {
		}
	}()
	t.Cleanup(func() { st.drop(); r.Stop() })
	clk.add(auditTimeout)
	st.hold()
	return r, st
}

// within returns f's result, or nil if f is still blocked after d (waiting
// for storage is the correct behaviour for a promise).
func within[T any](d time.Duration, f func() T) (T, bool) {
	ch := make(chan T, 1)
	go func() { ch <- f() }()
	select {
	case v := <-ch:
		return v, true
	case <-time.After(d):
		var zero T
		return zero, false
	}
}

func durableHS(m *MemStorage) HardState { hs, _, _, _ := m.Load(); return hs }

// A promise (granted vote, successful append) must not leave before the
// term and vote it is made in are stored.
func TestPromisesWaitForDurableTerm(t *testing.T) {
	base := func() *MemStorage {
		return preload(t, HardState{Term: 5, Vote: 2},
			Entry{Index: 1, Term: 5}, Entry{Index: 2, Term: 5}, Entry{Index: 3, Term: 5})
	}

	t.Run("vote", func(t *testing.T) {
		mem := base()
		r, _ := startAudit(t, mem)
		resp, done := within(200*time.Millisecond, func() *RequestVoteResponse {
			return r.HandleRequestVote(&RequestVoteRequest{Term: 6, Candidate: 3, LastLogIndex: 3, LastLogTerm: 5})
		})
		if done && resp.Granted {
			if hs := durableHS(mem); hs.Term < 6 || hs.Vote != 3 {
				t.Fatalf("vote in term 6 granted while stable hard state is %+v", hs)
			}
		}
	})

	t.Run("append", func(t *testing.T) {
		mem := base()
		r, _ := startAudit(t, mem)
		resp, done := within(200*time.Millisecond, func() *AppendEntriesResponse {
			return r.HandleAppendEntries(&AppendEntriesRequest{Term: 7, Leader: 4, PrevIndex: 3, PrevTerm: 5})
		})
		if done && resp.Success {
			if hs := durableHS(mem); hs.Term < 7 {
				t.Fatalf("append in term 7 acknowledged while stable hard state is %+v", hs)
			}
		}
	})
}

// Refusals may carry a term that is not yet stored. A crash then rolls the
// node back to its stored term, but the stored vote still binds it: it cannot
// vote twice in a term it voted in.
func TestRefusalsMayCarryUnstoredTerm(t *testing.T) {
	mem := preload(t, HardState{Term: 5, Vote: 2},
		Entry{Index: 1, Term: 5}, Entry{Index: 2, Term: 5}, Entry{Index: 3, Term: 5})
	r, st := startAudit(t, mem)

	vote, ok := within(200*time.Millisecond, func() *RequestVoteResponse {
		// Candidate's log is behind: refused, but the node adopts term 6.
		return r.HandleRequestVote(&RequestVoteRequest{Term: 6, Candidate: 3, LastLogIndex: 1, LastLogTerm: 4})
	})
	if !ok || vote.Granted || vote.Term != 6 {
		t.Fatalf("stale candidate: got %+v (returned %v), want an immediate refusal in term 6", vote, ok)
	}
	app, ok := within(200*time.Millisecond, func() *AppendEntriesResponse {
		// Gap in the log: refused, but the node adopts term 7.
		return r.HandleAppendEntries(&AppendEntriesRequest{Term: 7, Leader: 4, PrevIndex: 10, PrevTerm: 7})
	})
	if !ok || app.Success || app.Term != 7 {
		t.Fatalf("append with a gap: got %+v (returned %v), want an immediate refusal in term 7", app, ok)
	}
	if hs := durableHS(mem); hs.Term != 5 {
		t.Fatalf("stable term %d, want 5 (writes are held)", hs.Term)
	}
	t.Logf("refusals answered in terms 6 and 7 while the stable term was 5")

	st.drop()
	r.Stop()
	r2, _ := startAudit(t, mem)
	if got := r2.Status().Term; got != 5 {
		t.Fatalf("restarted in term %d, want 5", got)
	}
	resp, done := within(200*time.Millisecond, func() *RequestVoteResponse {
		return r2.HandleRequestVote(&RequestVoteRequest{Term: 5, Candidate: 3, LastLogIndex: 3, LastLogTerm: 5})
	})
	if done && resp.Granted {
		t.Fatal("after the rollback the node voted for 3 in term 5, where it had voted for 2")
	}
}

// The one success that does not wait: an append whose entries all lie inside
// the follower's snapshot. It reports a match only for entries the follower
// has already applied, i.e. committed ones, so it cannot help a leader commit
// anything new; the term it carries may not be stored yet. Recorded here so
// a change to this path is noticed.
func TestSnapshotCoveredAppendDoesNotWait(t *testing.T) {
	mem := NewMemStorage()
	if err := mem.SaveSnapshot(Snapshot{Index: 3, Term: 5}, HardState{Term: 5, Vote: 2}, nil); err != nil {
		t.Fatal(err)
	}
	r, _ := startAudit(t, mem)
	resp, ok := within(200*time.Millisecond, func() *AppendEntriesResponse {
		return r.HandleAppendEntries(&AppendEntriesRequest{Term: 7, Leader: 4, PrevIndex: 1, PrevTerm: 5,
			Entries: []Entry{{Index: 2, Term: 5}, {Index: 3, Term: 5}}})
	})
	if !ok {
		t.Skip("the snapshot-covered append now waits for storage; update RAFT-DEEP-DIVE §16.2")
	}
	if !resp.Success || resp.MatchIndex != 3 {
		t.Fatalf("got %+v, want success with match 3", resp)
	}
	if c := r.Status().Commit; resp.MatchIndex > c {
		t.Fatalf("acknowledged match %d beyond the follower's commit %d", resp.MatchIndex, c)
	}
	t.Logf("acknowledged match %d (<= commit %d) in term 7 while the stable term is %d",
		resp.MatchIndex, r.Status().Commit, durableHS(mem).Term)
}

// A follower applies committed entries even while their write to its own log
// is still pending (commit is advanced before the durability wait in
// HandleAppendEntries). That is safe because the entries are already stored
// on a majority: if the follower crashes before writing them, it reloads a
// shorter log, rebuilds its state machine from scratch and is sent the same
// entries again. Here F applies X while its storage is held, crashes, and
// after restarting applies the same command at the same index.
func TestFollowerAppliesBeforeStoringThenRecovers(t *testing.T) {
	const n = 5
	net := &network{nodes: map[uint64]*Raft{}, connected: map[uint64]bool{}, reliable: true}
	mems := make([]*MemStorage, n)
	stores := make([]*gatedStorage, n)
	rafts := make([]*Raft, n)
	applied := make([]map[uint64][]byte, n)
	var amu sync.Mutex
	start := func(i int) {
		stores[i] = newGated(mems[i])
		apply := make(chan ApplyMsg, 1024)
		r, err := New(Config{
			ID: id(i), Peers: []uint64{1, 2, 3, 4, 5},
			ElectionTimeout: testElection, HeartbeatInterval: testHeartbeat,
			Storage: stores[i], Transport: memTransport{net: net, from: id(i)}, Apply: apply,
		})
		if err != nil {
			t.Fatal(err)
		}
		amu.Lock()
		applied[i] = map[uint64][]byte{}
		m := applied[i]
		amu.Unlock()
		go func() {
			for msg := range apply {
				if msg.Entry != nil {
					amu.Lock()
					m[msg.Entry.Index] = msg.Entry.Data
					amu.Unlock()
				}
			}
		}()
		rafts[i] = r
		net.mu.Lock()
		net.nodes[id(i)] = r
		net.connected[id(i)] = true
		net.mu.Unlock()
	}
	crash := func(i int) {
		net.mu.Lock()
		net.connected[id(i)] = false
		net.nodes[id(i)] = nil
		net.mu.Unlock()
		stores[i].drop()
		rafts[i].Stop()
		rafts[i] = nil
	}
	t.Cleanup(func() {
		for i := range n {
			if rafts[i] != nil {
				crash(i)
			}
		}
	})
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	appliedAt := func(i int, ix uint64) ([]byte, bool) {
		amu.Lock()
		defer amu.Unlock()
		b, ok := applied[i][ix]
		return b, ok
	}

	for i := range n {
		mems[i] = NewMemStorage()
		start(i)
	}
	L := -1
	waitFor("a leader", func() bool {
		for i := range n {
			if rafts[i].Status().Role == Leader {
				L = i
				return true
			}
		}
		return false
	})
	F := (L + 1) % n
	stores[F].hold()
	const X = 7319
	ix, _, err := rafts[L].Propose(encodeCmd(X))
	if err != nil {
		t.Fatal(err)
	}
	waitFor("F to apply X from memory", func() bool { _, ok := appliedAt(F, ix); return ok })
	if got := storedLast(mems[F]); got >= ix {
		t.Fatalf("F stored up to %d; the test needs X (index %d) unstored", got, ix)
	}
	b, _ := appliedAt(F, ix)
	if decodeCmd(b) != X {
		t.Fatalf("F applied %v at %d, want X", b, ix)
	}

	crash(F)
	start(F)
	waitFor("F to apply index "+fmt.Sprint(ix)+" again after restart", func() bool { _, ok := appliedAt(F, ix); return ok })
	b, _ = appliedAt(F, ix)
	if decodeCmd(b) != X {
		t.Fatalf("after restart F applied %d at index %d, before the crash it applied %d", decodeCmd(b), ix, X)
	}
	for i := range n {
		if i == F {
			continue
		}
		if c, ok := appliedAt(i, ix); ok && decodeCmd(c) != X {
			t.Fatalf("node %d applied %d at index %d, F applied %d", i+1, decodeCmd(c), ix, X)
		}
	}
}
