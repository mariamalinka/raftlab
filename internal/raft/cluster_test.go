package raft

// Test cluster: an in-memory network that can disconnect, crash, drop, delay
// and reorder RPCs, plus checks that every node applies the same command at
// each index. The test list follows the structure of the MIT 6.5840 Raft lab
// tests, which the plan uses as the G1 gate.

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	flagPipeline = flag.Bool("pipeline", false, "run the raft tests with pipelined AppendEntries")
	flagBatch    = flag.Int("batch", 64, "MaxBatch for the raft tests (1 disables batching)")
)

const (
	testElection  = 100 * time.Millisecond
	testHeartbeat = 20 * time.Millisecond
)

var errDisconnected = errors.New("disconnected")

type network struct {
	mu        sync.Mutex
	nodes     map[uint64]*Raft
	connected map[uint64]bool
	reliable  bool
	reorder   bool
	rpcs      atomic.Int64
}

func (n *network) call(ctx context.Context, from, to uint64, h func(*Raft) any) (any, error) {
	n.rpcs.Add(1)
	n.mu.Lock()
	ok := n.connected[from] && n.connected[to]
	target := n.nodes[to]
	reliable, reorder := n.reliable, n.reorder
	n.mu.Unlock()
	if !ok || target == nil {
		select {
		case <-time.After(time.Duration(rand.Intn(10)) * time.Millisecond):
		case <-ctx.Done():
		}
		return nil, errDisconnected
	}
	if !reliable {
		time.Sleep(time.Duration(rand.Intn(27)) * time.Millisecond)
		if rand.Intn(10) == 0 {
			return nil, errDisconnected
		}
	}
	done := make(chan any, 1)
	go func() { done <- h(target) }()
	select {
	case resp := <-done:
		n.mu.Lock()
		ok := n.connected[from] && n.connected[to]
		n.mu.Unlock()
		if !ok || (!reliable && rand.Intn(10) == 0) {
			return nil, errDisconnected
		}
		if reorder && rand.Intn(900) < 600 {
			time.Sleep(time.Duration(200+rand.Intn(1+rand.Intn(2000))) * time.Millisecond)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type memTransport struct {
	net  *network
	from uint64
}

func (t memTransport) RequestVote(ctx context.Context, to uint64, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	r, err := t.net.call(ctx, t.from, to, func(n *Raft) any { return n.HandleRequestVote(req) })
	if err != nil {
		return nil, err
	}
	return r.(*RequestVoteResponse), nil
}

func (t memTransport) AppendEntries(ctx context.Context, to uint64, req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	r, err := t.net.call(ctx, t.from, to, func(n *Raft) any { return n.HandleAppendEntries(req) })
	if err != nil {
		return nil, err
	}
	return r.(*AppendEntriesResponse), nil
}

func (t memTransport) InstallSnapshot(ctx context.Context, to uint64, req *InstallSnapshotRequest) (*InstallSnapshotResponse, error) {
	r, err := t.net.call(ctx, t.from, to, func(n *Raft) any { return n.HandleInstallSnapshot(req) })
	if err != nil {
		return nil, err
	}
	return r.(*InstallSnapshotResponse), nil
}

type cluster struct {
	t        *testing.T
	n        int
	net      *network
	rafts    []*Raft
	storage  []*MemStorage
	applyErr []string
	logs     []map[uint64]int // per node: index -> command
	mu       sync.Mutex
	snapshot bool
	maxIndex uint64
	start    time.Time
	stops    []chan struct{}
}

func newCluster(t *testing.T, n int, reliable, snapshot bool) *cluster {
	c := &cluster{
		t:        t,
		n:        n,
		net:      &network{nodes: map[uint64]*Raft{}, connected: map[uint64]bool{}, reliable: reliable},
		rafts:    make([]*Raft, n),
		storage:  make([]*MemStorage, n),
		applyErr: make([]string, n),
		logs:     make([]map[uint64]int, n),
		snapshot: snapshot,
		start:    time.Now(),
		stops:    make([]chan struct{}, n),
	}
	for i := range n {
		c.storage[i] = NewMemStorage()
		c.start1(i)
		c.connect(i)
	}
	t.Cleanup(c.cleanup)
	return c
}

func (c *cluster) cleanup() {
	for i := range c.n {
		c.crash1(i)
	}
	for i, e := range c.applyErr {
		if e != "" {
			c.t.Errorf("node %d apply error: %s", i+1, e)
		}
	}
}

func id(i int) uint64 { return uint64(i + 1) }

func (c *cluster) peers() []uint64 {
	ps := make([]uint64, c.n)
	for i := range c.n {
		ps[i] = id(i)
	}
	return ps
}

func (c *cluster) start1(i int) {
	c.crash1(i)
	apply := make(chan ApplyMsg, 64)
	r, err := New(Config{
		ID:                id(i),
		Peers:             c.peers(),
		ElectionTimeout:   testElection,
		HeartbeatInterval: testHeartbeat,
		LeaseDriftBound:   0.1,
		MaxBatch:          *flagBatch,
		Pipeline:          *flagPipeline,
		Storage:           c.storage[i],
		Transport:         memTransport{net: c.net, from: id(i)},
		Apply:             apply,
	})
	if err != nil {
		c.t.Fatal(err)
	}
	c.mu.Lock()
	c.logs[i] = map[uint64]int{}
	c.rafts[i] = r
	stop := make(chan struct{})
	c.stops[i] = stop
	c.mu.Unlock()
	c.net.mu.Lock()
	c.net.nodes[id(i)] = r
	c.net.mu.Unlock()
	go c.applier(i, r, apply, stop)
}

func encodeCmd(v int) []byte { return binary.LittleEndian.AppendUint64(nil, uint64(v)) }
func decodeCmd(b []byte) int { return int(binary.LittleEndian.Uint64(b)) }

type testSnap struct {
	Index uint64
	Log   map[uint64]int
}

func (c *cluster) applier(i int, r *Raft, apply chan ApplyMsg, stop chan struct{}) {
	var applied uint64
	for {
		var m ApplyMsg
		select {
		case m = <-apply:
		case <-stop:
			return
		}
		if m.Snapshot != nil {
			var s testSnap
			if err := json.Unmarshal(m.Snapshot.Data, &s); err != nil {
				c.fail(i, fmt.Sprintf("bad snapshot: %v", err))
				return
			}
			c.mu.Lock()
			c.logs[i] = s.Log
			c.mu.Unlock()
			applied = m.Snapshot.Index
			continue
		}
		e := m.Entry
		if e.Index != applied+1 {
			c.fail(i, fmt.Sprintf("applied index %d after %d", e.Index, applied))
			return
		}
		applied = e.Index
		if e.Type == EntryNoop {
			continue
		}
		v := decodeCmd(e.Data)
		c.mu.Lock()
		for j := range c.n {
			if old, ok := c.logs[j][e.Index]; ok && old != v {
				c.mu.Unlock()
				c.fail(i, fmt.Sprintf("commit index=%d node=%d %v != node=%d %v", e.Index, i+1, v, j+1, old))
				return
			}
		}
		c.logs[i][e.Index] = v
		if e.Index > c.maxIndex {
			c.maxIndex = e.Index
		}
		c.mu.Unlock()
		if c.snapshot && e.Index%10 == 0 {
			c.mu.Lock()
			data, _ := json.Marshal(testSnap{Index: e.Index, Log: c.logs[i]})
			c.mu.Unlock()
			r.Snapshot(e.Index, data)
		}
	}
}

func (c *cluster) fail(i int, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.applyErr[i] == "" {
		c.applyErr[i] = msg
	}
}

func (c *cluster) checkApplyErrors() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, e := range c.applyErr {
		if e != "" {
			c.t.Fatalf("node %d: %s", i+1, e)
		}
	}
}

func (c *cluster) crash1(i int) {
	c.disconnect(i)
	c.mu.Lock()
	r := c.rafts[i]
	c.rafts[i] = nil
	stop := c.stops[i]
	c.mu.Unlock()
	if r != nil {
		c.net.mu.Lock()
		c.net.nodes[id(i)] = nil
		c.net.mu.Unlock()
		close(stop)
		r.Stop()
		c.storage[i].Crash()
	}
}

func (c *cluster) connect(i int) {
	c.net.mu.Lock()
	c.net.connected[id(i)] = true
	c.net.mu.Unlock()
}

func (c *cluster) disconnect(i int) {
	c.net.mu.Lock()
	c.net.connected[id(i)] = false
	c.net.mu.Unlock()
}

func (c *cluster) isConnected(i int) bool {
	c.net.mu.Lock()
	defer c.net.mu.Unlock()
	return c.net.connected[id(i)]
}

func (c *cluster) setReliable(v bool) {
	c.net.mu.Lock()
	c.net.reliable = v
	c.net.mu.Unlock()
}

func (c *cluster) setReorder(v bool) {
	c.net.mu.Lock()
	c.net.reorder = v
	c.net.mu.Unlock()
}

func (c *cluster) raft(i int) *Raft {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rafts[i]
}

// checkOneLeader waits for exactly one leader among connected nodes.
func (c *cluster) checkOneLeader() int {
	for range 10 {
		time.Sleep(time.Duration(450+rand.Intn(100)) * time.Millisecond)
		leaders := map[uint64][]int{}
		for i := range c.n {
			if r := c.raft(i); r != nil && c.isConnected(i) {
				if s := r.Status(); s.Role == Leader {
					leaders[s.Term] = append(leaders[s.Term], i)
				}
			}
		}
		var last uint64
		for term, ls := range leaders {
			if len(ls) > 1 {
				c.t.Fatalf("term %d has %d leaders", term, len(ls))
			}
			last = max(last, term)
		}
		if len(leaders) != 0 {
			return leaders[last][0]
		}
	}
	c.t.Fatal("expected one leader, got none")
	return -1
}

func (c *cluster) checkTerms() uint64 {
	var term uint64
	for i := range c.n {
		if r := c.raft(i); r != nil && c.isConnected(i) {
			t := r.Status().Term
			if term == 0 {
				term = t
			} else if term != t {
				c.t.Fatal("servers disagree on term")
			}
		}
	}
	return term
}

func (c *cluster) checkNoLeader() {
	for i := range c.n {
		if r := c.raft(i); r != nil && c.isConnected(i) && r.Status().Role == Leader {
			c.t.Fatalf("expected no leader among connected servers, but %d claims", i+1)
		}
	}
}

// nCommitted counts how many nodes have applied index, checking they agree.
func (c *cluster) nCommitted(index uint64) (int, int) {
	c.checkApplyErrors()
	c.mu.Lock()
	defer c.mu.Unlock()
	count, cmd := 0, -1
	for i := range c.n {
		if v, ok := c.logs[i][index]; ok {
			if count > 0 && v != cmd {
				c.t.Fatalf("committed values do not match: index %d, %v, %v", index, cmd, v)
			}
			count++
			cmd = v
		}
	}
	return count, cmd
}

// propose submits cmd to node i, returning the index if i is leader.
func (c *cluster) propose(i int, cmd int) (uint64, uint64, bool) {
	r := c.raft(i)
	if r == nil {
		return 0, 0, false
	}
	idx, term, err := r.Propose(encodeCmd(cmd))
	return idx, term, err == nil
}

// one submits cmd until some leader commits it on at least expected nodes.
func (c *cluster) one(cmd, expected int, retry bool) uint64 {
	deadline := time.Now().Add(10 * time.Second)
	starts := 0
	for time.Now().Before(deadline) {
		var index uint64
		found := false
		for range c.n {
			starts = (starts + 1) % c.n
			if c.isConnected(starts) {
				if idx, _, ok := c.propose(starts, cmd); ok {
					index, found = idx, true
					break
				}
			}
		}
		if found {
			t1 := time.Now()
			for time.Since(t1) < 2*time.Second {
				nd, cmd1 := c.nCommitted(index)
				if nd > 0 && nd >= expected && cmd1 == cmd {
					return index
				}
				time.Sleep(20 * time.Millisecond)
			}
			if !retry {
				c.t.Fatalf("one(%v) failed to reach agreement", cmd)
			}
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	c.t.Fatalf("one(%v) failed to reach agreement", cmd)
	return 0
}

// wait for at least n nodes to commit index, but don't wait forever.
func (c *cluster) wait(index uint64, n int, startTerm uint64) int {
	to := 10 * time.Millisecond
	for range 30 {
		nd, _ := c.nCommitted(index)
		if nd >= n {
			break
		}
		time.Sleep(to)
		if to < time.Second {
			to *= 2
		}
		if startTerm > 0 {
			for i := range c.n {
				if r := c.raft(i); r != nil && r.Status().Term > startTerm {
					return -1
				}
			}
		}
	}
	nd, cmd := c.nCommitted(index)
	if nd < n {
		c.t.Fatalf("only %d decided for index %d; wanted %d", nd, index, n)
	}
	return cmd
}
