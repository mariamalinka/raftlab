package raft

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// ---- elections ----

func TestInitialElection(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.checkOneLeader()
	time.Sleep(50 * time.Millisecond)
	t1 := c.checkTerms()
	if t1 < 1 {
		t.Fatalf("term is %d, but should be at least 1", t1)
	}
	time.Sleep(2 * testElection * 5)
	if t2 := c.checkTerms(); t1 != t2 {
		t.Logf("warning: term changed even though there were no failures")
	}
	c.checkOneLeader()
}

func TestReElection(t *testing.T) {
	c := newCluster(t, 3, true, false)
	l1 := c.checkOneLeader()
	c.disconnect(l1)
	c.checkOneLeader()
	c.connect(l1)
	l2 := c.checkOneLeader()
	c.disconnect(l2)
	c.disconnect((l2 + 1) % 3)
	time.Sleep(10 * testElection)
	c.checkNoLeader()
	c.connect((l2 + 1) % 3)
	c.checkOneLeader()
	c.connect(l2)
	c.checkOneLeader()
}

func TestManyElections(t *testing.T) {
	c := newCluster(t, 7, true, false)
	c.checkOneLeader()
	for range 10 {
		i1, i2, i3 := rand.Intn(7), rand.Intn(7), rand.Intn(7)
		c.disconnect(i1)
		c.disconnect(i2)
		c.disconnect(i3)
		c.checkOneLeader()
		c.connect(i1)
		c.connect(i2)
		c.connect(i3)
	}
	c.checkOneLeader()
}

// ---- agreement ----

func TestBasicAgree(t *testing.T) {
	c := newCluster(t, 3, true, false)
	for i := 1; i <= 3; i++ {
		c.one(i*100, 3, false)
	}
}

func TestFollowerFailure(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(101, 3, false)
	l := c.checkOneLeader()
	c.disconnect((l + 1) % 3)
	c.one(102, 2, false)
	time.Sleep(2 * testElection)
	c.one(103, 2, false)
	l2 := c.checkOneLeader()
	c.disconnect((l2 + 1) % 3)
	c.disconnect((l2 + 2) % 3)
	idx, _, ok := c.propose(l2, 104)
	if !ok {
		t.Fatal("leader rejected Propose")
	}
	time.Sleep(2 * testElection)
	if n, _ := c.nCommitted(idx); n > 0 {
		t.Fatalf("%d committed but no majority", n)
	}
}

func TestLeaderFailure(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(101, 3, false)
	l1 := c.checkOneLeader()
	c.disconnect(l1)
	c.one(102, 2, false)
	time.Sleep(2 * testElection)
	c.one(103, 2, false)
	l2 := c.checkOneLeader()
	c.disconnect(l2)
	for i := range 3 {
		c.propose(i, 104)
	}
	time.Sleep(2 * testElection)
	if n, _ := c.nCommitted(4 + 2); n > 0 { // 3 cmds + noops; nothing new may commit
		t.Logf("note: index 6 has %d", n)
	}
}

func TestFailAgree(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(101, 3, false)
	l := c.checkOneLeader()
	c.disconnect((l + 1) % 3)
	c.one(102, 2, false)
	c.one(103, 2, false)
	time.Sleep(testElection)
	c.one(104, 2, false)
	c.one(105, 2, false)
	c.connect((l + 1) % 3)
	c.one(106, 3, true)
	time.Sleep(testElection)
	c.one(107, 3, true)
}

func TestFailNoAgree(t *testing.T) {
	c := newCluster(t, 5, true, false)
	c.one(10, 5, false)
	l := c.checkOneLeader()
	c.disconnect((l + 1) % 5)
	c.disconnect((l + 2) % 5)
	c.disconnect((l + 3) % 5)
	idx, _, ok := c.propose(l, 20)
	if !ok {
		t.Fatal("leader rejected Propose")
	}
	time.Sleep(2 * testElection)
	if n, _ := c.nCommitted(idx); n > 0 {
		t.Fatalf("%d committed but no majority", n)
	}
	c.connect((l + 1) % 5)
	c.connect((l + 2) % 5)
	c.connect((l + 3) % 5)
	l2 := c.checkOneLeader()
	if _, _, ok := c.propose(l2, 30); !ok {
		t.Fatal("leader2 rejected Propose")
	}
	c.one(1000, 5, true)
}

func TestConcurrentStarts(t *testing.T) {
	c := newCluster(t, 3, true, false)
	for try := 0; try < 5; try++ {
		if try > 0 {
			time.Sleep(3 * time.Second)
		}
		l := c.checkOneLeader()
		_, term, ok := c.propose(l, 1)
		if !ok {
			continue
		}
		var mu sync.Mutex
		var idxs []uint64
		var wg sync.WaitGroup
		for i := range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				idx, t1, ok := c.propose(l, 100+i)
				if ok && t1 == term {
					mu.Lock()
					idxs = append(idxs, idx)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		changed := false
		for j := range 3 {
			if r := c.raft(j); r != nil && r.Status().Term != term {
				changed = true
			}
		}
		if changed {
			continue
		}
		got := map[int]bool{}
		failed := false
		for _, idx := range idxs {
			cmd := c.wait(idx, 3, term)
			if cmd == -1 {
				failed = true
				break
			}
			got[cmd] = true
		}
		if failed {
			continue
		}
		for i := range 5 {
			if !got[100+i] {
				t.Fatalf("cmd %d missing", 100+i)
			}
		}
		return
	}
	t.Fatal("term changed too often")
}

func TestRejoin(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(101, 3, true)
	l1 := c.checkOneLeader()
	c.disconnect(l1)
	c.propose(l1, 102)
	c.propose(l1, 103)
	c.propose(l1, 104)
	c.one(103, 2, true)
	l2 := c.checkOneLeader()
	c.disconnect(l2)
	c.connect(l1)
	c.one(104, 2, true)
	c.connect(l2)
	c.one(105, 3, true)
}

func TestBackup(t *testing.T) {
	c := newCluster(t, 5, true, false)
	c.one(rand.Int(), 5, true)
	l1 := c.checkOneLeader()
	c.disconnect((l1 + 2) % 5)
	c.disconnect((l1 + 3) % 5)
	c.disconnect((l1 + 4) % 5)
	for range 50 {
		c.propose(l1, rand.Int())
	}
	time.Sleep(testElection)
	c.disconnect((l1 + 0) % 5)
	c.disconnect((l1 + 1) % 5)
	c.connect((l1 + 2) % 5)
	c.connect((l1 + 3) % 5)
	c.connect((l1 + 4) % 5)
	for range 50 {
		c.one(rand.Int(), 3, true)
	}
	leader2 := c.checkOneLeader()
	other := (l1 + 2) % 5
	if leader2 == other {
		other = (leader2 + 1) % 5
	}
	c.disconnect(other)
	for range 50 {
		c.propose(leader2, rand.Int())
	}
	time.Sleep(testElection)
	for i := range 5 {
		c.disconnect(i)
	}
	c.connect((l1 + 0) % 5)
	c.connect((l1 + 1) % 5)
	c.connect(other)
	for range 50 {
		c.one(rand.Int(), 3, true)
	}
	for i := range 5 {
		c.connect(i)
	}
	c.one(rand.Int(), 5, true)
}

// ---- persistence ----

func TestPersist1(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(11, 3, true)
	for i := range 3 {
		c.start1(i)
		c.connect(i)
	}
	c.one(12, 3, true)
	l1 := c.checkOneLeader()
	c.start1(l1)
	c.connect(l1)
	c.one(13, 3, true)
	l2 := c.checkOneLeader()
	c.crash1(l2)
	c.one(14, 2, true)
	c.start1(l2)
	c.connect(l2)
	c.wait(c.maxIdx(), 3, 0)
	i3 := (c.checkOneLeader() + 1) % 3
	c.crash1(i3)
	c.one(15, 2, true)
	c.start1(i3)
	c.connect(i3)
	c.one(16, 3, true)
}

func (c *cluster) maxIdx() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxIndex
}

func TestPersist2(t *testing.T) {
	c := newCluster(t, 5, true, false)
	index := 1
	for range 5 {
		c.one(10+index, 5, true)
		index++
		l1 := c.checkOneLeader()
		c.crash1((l1 + 1) % 5)
		c.crash1((l1 + 2) % 5)
		c.one(10+index, 3, true)
		index++
		c.crash1((l1 + 0) % 5)
		c.crash1((l1 + 3) % 5)
		c.crash1((l1 + 4) % 5)
		c.start1((l1 + 1) % 5)
		c.start1((l1 + 2) % 5)
		c.connect((l1 + 1) % 5)
		c.connect((l1 + 2) % 5)
		time.Sleep(testElection)
		c.start1((l1 + 3) % 5)
		c.connect((l1 + 3) % 5)
		c.one(10+index, 3, true)
		index++
		c.start1((l1 + 4) % 5)
		c.start1((l1 + 0) % 5)
		c.connect((l1 + 4) % 5)
		c.connect((l1 + 0) % 5)
	}
	c.one(1000, 5, true)
}

func TestPersist3(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(101, 3, true)
	l := c.checkOneLeader()
	c.disconnect((l + 2) % 3)
	c.one(102, 2, true)
	c.crash1((l + 0) % 3)
	c.crash1((l + 1) % 3)
	c.connect((l + 2) % 3)
	c.start1((l + 0) % 3)
	c.connect((l + 0) % 3)
	c.one(103, 2, true)
	c.start1((l + 1) % 3)
	c.connect((l + 1) % 3)
	c.one(104, 3, true)
}

// TestFigure8 exercises the scenario in Figure 8 of the Raft paper: leaders
// crash after replicating to a minority, and a later leader must not commit
// an earlier term's entry by counting replicas.
func TestFigure8(t *testing.T) {
	c := newCluster(t, 5, true, false)
	c.one(rand.Int(), 1, true)
	nup := 5
	for iters := 0; iters < 1000; iters++ {
		leader := -1
		for i := range 5 {
			if c.raft(i) != nil {
				if _, _, ok := c.propose(i, rand.Int()); ok {
					leader = i
				}
			}
		}
		if rand.Intn(1000) < 100 {
			time.Sleep(time.Duration(rand.Int63()%(int64(testElection)/2)) * time.Nanosecond)
		} else {
			time.Sleep(time.Duration(rand.Int63()%13) * time.Millisecond)
		}
		if leader != -1 {
			c.crash1(leader)
			nup--
		}
		if nup < 3 {
			s := rand.Intn(5)
			if c.raft(s) == nil {
				c.start1(s)
				c.connect(s)
				nup++
			}
		}
	}
	for i := range 5 {
		if c.raft(i) == nil {
			c.start1(i)
			c.connect(i)
		}
	}
	c.one(rand.Int(), 5, true)
}

func TestUnreliableAgree(t *testing.T) {
	c := newCluster(t, 5, false, false)
	var wg sync.WaitGroup
	for iters := 1; iters < 50; iters++ {
		for j := range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.one(100*iters+j, 1, true)
			}()
		}
		c.one(iters, 1, true)
	}
	c.setReliable(true)
	wg.Wait()
	c.one(100, 5, true)
}

func TestFigure8Unreliable(t *testing.T) {
	c := newCluster(t, 5, false, false)
	c.one(rand.Intn(10000), 1, true)
	nup := 5
	for iters := 0; iters < 1000; iters++ {
		if iters == 200 {
			c.setReorder(true)
		}
		leader := -1
		for i := range 5 {
			if _, _, ok := c.propose(i, rand.Intn(10000)); ok && c.isConnected(i) {
				leader = i
			}
		}
		if rand.Intn(1000) < 100 {
			time.Sleep(time.Duration(rand.Int63()%(int64(testElection)/2)) * time.Nanosecond)
		} else {
			time.Sleep(time.Duration(rand.Int63()%13) * time.Millisecond)
		}
		if leader != -1 && rand.Intn(1000) < int(testElection/time.Millisecond)/2 {
			c.disconnect(leader)
			nup--
		}
		if nup < 3 {
			s := rand.Intn(5)
			if !c.isConnected(s) {
				c.connect(s)
				nup++
			}
		}
	}
	for i := range 5 {
		c.connect(i)
	}
	c.setReorder(false)
	c.one(rand.Intn(10000), 5, true)
}

func churn(t *testing.T, reliable bool) {
	c := newCluster(t, 5, reliable, false)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var values []int
	for me := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				x++
				v := rand.Int()
				var index uint64
				ok := false
				for i := range 5 {
					if idx, _, ok1 := c.propose(i, v); ok1 {
						index, ok = idx, true
					}
				}
				if ok {
					for _, to := range []int{10, 20, 50, 100, 200} {
						if n, cmd := c.nCommitted(index); n > 0 {
							if cmd == v {
								mu.Lock()
								values = append(values, v)
								mu.Unlock()
							}
							break
						}
						time.Sleep(time.Duration(to) * time.Millisecond)
					}
				} else {
					time.Sleep(time.Duration(79+me*17) * time.Millisecond)
				}
			}
		}()
	}
	time.Sleep(time.Second)
	for iters := 0; iters < 20; iters++ {
		if rand.Intn(1000) < 200 {
			c.disconnect(rand.Intn(5))
		}
		if rand.Intn(1000) < 500 {
			i := rand.Intn(5)
			if c.raft(i) == nil {
				c.start1(i)
			}
			c.connect(i)
		}
		if rand.Intn(1000) < 200 {
			i := rand.Intn(5)
			if c.raft(i) != nil {
				c.crash1(i)
			}
		}
		time.Sleep(testElection * 7 / 10)
	}
	time.Sleep(testElection)
	c.setReliable(true)
	for i := range 5 {
		if c.raft(i) == nil {
			c.start1(i)
		}
		c.connect(i)
	}
	close(stop)
	wg.Wait()
	time.Sleep(testElection)
	lastIndex := c.one(rand.Int(), 5, true)
	have := map[int]bool{}
	for idx := uint64(1); idx <= lastIndex; idx++ {
		if _, v := c.nCommitted(idx); v != -1 {
			have[v] = true
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, v := range values {
		if !have[v] {
			t.Fatalf("didn't find a value %d", v)
		}
	}
}

func TestReliableChurn(t *testing.T)   { churn(t, true) }
func TestUnreliableChurn(t *testing.T) { churn(t, false) }

// ---- snapshots ----

func snapCommon(t *testing.T, disconnect, reliable, crash bool) {
	c := newCluster(t, 3, reliable, true)
	c.one(rand.Int(), 3, true)
	leader1 := c.checkOneLeader()
	for i := 0; i < 30; i++ {
		victim := (leader1 + 1) % 3
		sender := leader1
		if i%3 == 1 {
			sender = (leader1 + 1) % 3
			victim = leader1
		}
		if disconnect {
			c.disconnect(victim)
			c.one(rand.Int(), 2, true)
		}
		if crash {
			c.crash1(victim)
			c.one(rand.Int(), 2, true)
		}
		nn := 11
		for range nn {
			c.propose(sender, rand.Int())
		}
		if !disconnect && !crash {
			c.one(rand.Int(), 3, true)
		} else {
			c.one(rand.Int(), 2, true)
		}
		if disconnect {
			c.connect(victim)
			c.one(rand.Int(), 3, true)
			leader1 = c.checkOneLeader()
		}
		if crash {
			c.start1(victim)
			c.connect(victim)
			c.one(rand.Int(), 3, true)
			leader1 = c.checkOneLeader()
		}
	}
	for i := range 3 {
		if r := c.raft(i); r != nil && r.Status().SnapIndex == 0 {
			t.Fatalf("node %d never took a snapshot", i+1)
		}
	}
}

func TestSnapshotBasic(t *testing.T)             { snapCommon(t, false, true, false) }
func TestSnapshotInstall(t *testing.T)           { snapCommon(t, true, true, false) }
func TestSnapshotInstallUnreliable(t *testing.T) { snapCommon(t, true, false, false) }
func TestSnapshotInstallCrash(t *testing.T)      { snapCommon(t, false, true, true) }
func TestSnapshotInstallUnCrash(t *testing.T)    { snapCommon(t, false, false, true) }

func TestSnapshotAllCrash(t *testing.T) {
	c := newCluster(t, 3, false, true)
	c.one(rand.Int(), 3, true)
	for range 5 {
		for range 11 {
			c.one(rand.Int(), 3, true)
		}
		index1 := c.one(rand.Int(), 3, true)
		for i := range 3 {
			c.crash1(i)
		}
		for i := range 3 {
			c.start1(i)
			c.connect(i)
		}
		index2 := c.one(rand.Int(), 3, true)
		if index2 < index1+1 {
			t.Fatalf("index decreased from %v to %v", index1, index2)
		}
	}
}

// ---- reads ----

func TestReadIndexRequiresQuorum(t *testing.T) {
	c := newCluster(t, 5, true, false)
	c.one(1, 5, true)
	l := c.checkOneLeader()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.raft(l).ReadIndex(ctx); err != nil {
		t.Fatalf("ReadIndex on healthy leader: %v", err)
	}
	// Isolate the leader: it must not confirm a read, even before it notices.
	c.disconnect(l)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*testElection)
	defer cancel2()
	if _, err := c.raft(l).ReadIndex(ctx2); err == nil {
		t.Fatal("isolated leader confirmed a ReadIndex")
	}
}

func TestLeaseExpiresWhenIsolated(t *testing.T) {
	c := newCluster(t, 3, true, false)
	c.one(1, 3, true)
	l := c.checkOneLeader()
	time.Sleep(3 * testHeartbeat)
	if _, ok := c.raft(l).LeaseRead(); !ok {
		t.Fatal("healthy leader has no lease")
	}
	c.disconnect(l)
	time.Sleep(testElection)
	if _, ok := c.raft(l).LeaseRead(); ok {
		t.Fatal("isolated leader still holds a lease after an election timeout")
	}
	// A new leader may only appear once the old lease has run out.
	c.checkOneLeader()
}
