package raft

import (
	"sync"
	"testing"
	"time"
)

// Deterministic tests for the two injected Raft bugs that bugs_test.go does
// not cover. Like those tests, each passes in a normal build and fails when
// its bug is switched on:
//
//	RAFTKV_BUGS=ackbeforefsync go test -tags bugs ./internal/raft -run TestBugAckBeforeFsync  # fails
//	RAFTKV_BUGS=leasenodrift go test -tags bugs ./internal/raft -run TestBugLeaseNoDrift      # fails

// syncGate is a MemStorage whose Sync can be held back, so a test can
// look at the node while a write has reached storage but is not yet durable.
type syncGate struct {
	*MemStorage
	mu   sync.Mutex
	gate chan struct{} // nil: Sync runs at once
}

func (g *syncGate) Sync() error {
	g.mu.Lock()
	ch := g.gate
	g.mu.Unlock()
	if ch != nil {
		<-ch
	}
	return g.MemStorage.Sync()
}

func (g *syncGate) hold() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gate == nil {
		g.gate = make(chan struct{})
	}
}

func (g *syncGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gate != nil {
		close(g.gate)
		g.gate = nil
	}
}

// ackbeforefsync: a leader may count its own copy of an entry towards the
// commit quorum only once that copy is durable (Figure 2: respond after
// stable storage). Here a one-node cluster's fsync is held back; committing
// the entry anyway means acknowledging a write a crash would lose.
func TestBugAckBeforeFsync(t *testing.T) {
	st := &syncGate{MemStorage: NewMemStorage()}
	apply := make(chan ApplyMsg, 64)
	r, err := New(Config{
		ID:              1,
		Peers:           []uint64{1},
		ElectionTimeout: 50 * time.Millisecond,
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
	t.Cleanup(func() { st.release(); r.Stop(); close(stop) })

	deadline := time.Now().Add(10 * time.Second)
	for {
		s := r.Status()
		if s.Role == Leader && s.Commit > 0 && s.Commit == s.LastIndex {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no leader with a committed no-op: %+v", s)
		}
		time.Sleep(5 * time.Millisecond)
	}

	st.hold()
	idx, _, err := r.Propose([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if c := r.Status().Commit; c >= idx {
		t.Fatalf("index %d committed (commit %d) while its fsync was still pending: the write is acknowledged but a crash would lose it", idx, c)
	}
}

// leasenodrift: a lease must be shorter than the election timeout by the
// drift bound, because the leader's clock may run slower than its
// followers' (thesis §6.4.1). Here a majority last acknowledged the leader
// 0.95 election timeouts ago. With a 10% drift bound the lease has expired;
// the buggy leader still trusts it.
func TestBugLeaseNoDrift(t *testing.T) {
	const timeout = time.Minute
	st := preload(t, HardState{Term: 4, Vote: 1}, Entry{Index: 1, Term: 4})
	apply := make(chan ApplyMsg, 64)
	r, err := New(Config{
		ID:              1,
		Peers:           []uint64{1, 2, 3, 4, 5},
		ElectionTimeout: timeout,
		LeaseDriftBound: 0.1,
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

	// lead makes node 1 the term-4 leader whose entry 1 is committed and
	// whose followers 2 and 3 last acknowledged it age ago.
	lead := func(age time.Duration) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.role = Leader
		r.leader = r.id
		r.commit = 1
		acked := r.clock.Now().Add(-age)
		r.prs = map[uint64]*progress{
			2: {match: 1, next: 2, ackSent: acked}, 3: {match: 1, next: 2, ackSent: acked},
			4: {match: 0, next: 1}, 5: {match: 0, next: 1},
		}
	}

	lead(timeout / 2)
	if _, ok := r.LeaseRead(); !ok {
		t.Fatal("lease not valid half an election timeout after a quorum acknowledged the leader")
	}
	lead(timeout * 95 / 100)
	if _, ok := r.LeaseRead(); ok {
		t.Fatal("lease still valid 0.95 election timeouts after the last quorum acknowledgement, beyond the 10% drift bound")
	}
}
