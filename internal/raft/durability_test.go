package raft

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// Regression tests for the acknowledged-write loss seen under the
// bridge-churn fault (results/faults-2026-10-10/correct/bridge-churn/trial-001,
// docs/INVESTIGATION-lost-write.md). A follower must not report an entry as
// matched before it is in stable storage (Figure 2: "respond to RPCs only
// after updating stable storage"); otherwise a leader can commit an entry
// that a crash then removes from the majority that counted it.

// gatedStorage wraps a MemStorage whose Append calls can be held, to model a
// follower whose persister has not yet written the entries it holds in
// memory. drop() makes held and future Appends fail without writing
// anything, as if the process died before the write.
type gatedStorage struct {
	*MemStorage
	mu      sync.Mutex
	cond    *sync.Cond
	closed  bool
	dropped bool
}

var errDropped = errors.New("test: process crashed before the write")

func newGated(m *MemStorage) *gatedStorage {
	g := &gatedStorage{MemStorage: m}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *gatedStorage) hold() { g.mu.Lock(); g.closed = true; g.mu.Unlock() }

func (g *gatedStorage) drop() {
	g.mu.Lock()
	g.dropped = true
	g.cond.Broadcast()
	g.mu.Unlock()
}

func (g *gatedStorage) Append(ents []Entry) error {
	g.mu.Lock()
	for g.closed && !g.dropped {
		g.cond.Wait()
	}
	dropped := g.dropped
	g.mu.Unlock()
	if dropped {
		return errDropped
	}
	return g.MemStorage.Append(ents)
}

// storedLast is the last index the underlying storage holds, i.e. what
// survives a crash.
func storedLast(m *MemStorage) uint64 {
	_, snap, ents, _ := m.Load()
	if len(ents) == 0 {
		return snap.Index
	}
	return ents[len(ents)-1].Index
}

// A follower that holds entries in memory while their write is still
// pending must not acknowledge them, whether the leader retransmits them or
// sends a heartbeat that points past them.
func TestFollowerAckRequiresStableStorage(t *testing.T) {
	mem := NewMemStorage()
	st := newGated(mem)
	apply := make(chan ApplyMsg, 64)
	r, err := New(Config{ID: 1, Peers: []uint64{1, 2, 3, 4, 5}, ElectionTimeout: time.Minute,
		Storage: st, Transport: downTransport{}, Apply: apply})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.drop(); r.Stop() })

	st.hold()
	first := &AppendEntriesRequest{Term: 1, Leader: 2, Entries: []Entry{{Index: 1, Term: 1}, {Index: 2, Term: 1}}}
	go r.HandleAppendEntries(first) // blocks until the entries are stored
	deadline := time.Now().Add(5 * time.Second)
	for r.Status().LastIndex < 2 {
		if time.Now().After(deadline) {
			t.Fatal("follower never received the entries")
		}
		time.Sleep(time.Millisecond)
	}

	for _, req := range []*AppendEntriesRequest{
		{Term: 1, Leader: 2, PrevIndex: 0, Entries: first.Entries}, // retransmission
		{Term: 1, Leader: 2, PrevIndex: 2, PrevTerm: 1},            // heartbeat past them
	} {
		done := make(chan *AppendEntriesResponse, 1)
		go func() { done <- r.HandleAppendEntries(req) }()
		select {
		case resp := <-done:
			if stored := storedLast(mem); resp.Success && resp.MatchIndex > stored {
				t.Fatalf("follower acknowledged match %d (prev %d, %d entries) but stable storage ends at %d",
					resp.MatchIndex, req.PrevIndex, len(req.Entries), stored)
			}
		case <-time.After(200 * time.Millisecond):
			// Waiting for the write is correct behaviour.
		}
	}
}

// End to end: a write acknowledged by the leader must survive a crash of the
// follower that completed its majority, a change of leader and log
// reconciliation. Five nodes: leader L, followers F and G, and S1, S2 cut
// off beforehand so they lag. F's writes are held, as on the bridge node
// that kept crashing in the failing trial.
func TestCommittedEntrySurvivesFollowerCrash(t *testing.T) {
	const n = 5
	net := &network{nodes: map[uint64]*Raft{}, connected: map[uint64]bool{}, reliable: true}
	mems := make([]*MemStorage, n)
	stores := make([]*gatedStorage, n)
	rafts := make([]*Raft, n)
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
		go func() {
			for range apply {
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
	setConnected := func(i int, v bool) {
		net.mu.Lock()
		net.connected[id(i)] = v
		net.mu.Unlock()
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
	leaderAmong := func(ids ...int) int {
		for _, i := range ids {
			if rafts[i] != nil && rafts[i].Status().Role == Leader {
				return i
			}
		}
		return -1
	}

	for i := range n {
		mems[i] = NewMemStorage()
		start(i)
	}
	all := []int{0, 1, 2, 3, 4}
	waitFor("a leader", func() bool { return leaderAmong(all...) >= 0 })
	L := leaderAmong(all...)
	first, _, err := rafts[L].Propose(encodeCmd(1))
	if err != nil {
		t.Fatal(err)
	}
	waitFor("entry 1 on every node", func() bool {
		for i := range n {
			if storedLast(mems[i]) < first {
				return false
			}
		}
		return true
	})

	var others []int
	for i := range n {
		if i != L {
			others = append(others, i)
		}
	}
	F, G, S1, S2 := others[0], others[1], others[2], others[3]
	setConnected(S1, false)
	setConnected(S2, false)
	stores[F].hold()

	const X = 4581
	ix, _, err := rafts[L].Propose(encodeCmd(X))
	if err != nil {
		t.Fatal(err)
	}
	// L and G store X; F holds it in memory only. X may only commit if F
	// acknowledges it, which it must not do before storing it.
	committed := false
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if rafts[L].Status().Commit >= ix {
			committed = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !committed {
		t.Logf("leader did not commit index %d while node %d could not store it (correct)", ix, F+1)
		return
	}
	t.Logf("leader %d committed X=%d at index %d; stable storage: L=%d G=%d F=%d",
		L+1, X, ix, storedLast(mems[L]), storedLast(mems[G]), storedLast(mems[F]))

	// F crashes before writing X, the two nodes that hold X go down, and F
	// returns alongside the two lagging nodes: a majority without X.
	crash(F)
	crash(L)
	crash(G)
	start(F)
	setConnected(S1, true)
	setConnected(S2, true)
	waitFor("a leader among F, S1, S2", func() bool { return leaderAmong(F, S1, S2) >= 0 })
	NL := leaderAmong(F, S1, S2)
	waitFor("the new leader to commit in its term", func() bool {
		st := rafts[NL].Status()
		return st.Commit >= ix
	})
	r := rafts[NL]
	r.mu.Lock()
	var got []byte
	if ix <= r.lastIndex() && ix > r.snapIndex() {
		got = r.entryAt(ix).Data
	}
	r.mu.Unlock()
	if len(got) != 8 || decodeCmd(got) != X {
		t.Fatalf("acknowledged write X=%d committed at index %d was lost: new leader %d (term %d) has %v there",
			X, ix, NL+1, r.Status().Term, got)
	}
}
