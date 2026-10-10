// Package raft is a from-scratch implementation of the Raft consensus
// algorithm (Ongaro & Ousterhout, Figure 2), with the extensions the research
// plan measures: ReadIndex and lease-based reads (thesis §6.4), batching,
// pipelined AppendEntries and a configurable fsync discipline. Each injectable
// bug from the plan's correctness table is guarded by a switch in package bugs.
//
// Concurrency model: one mutex guards all state. Disk writes happen on a
// single persister goroutine in the order they were queued, so slow fsyncs
// never hold the lock. RPC handlers that must not answer before their writes
// are durable wait on the persister.
package raft

import (
	"context"
	"log/slog"
	"math/rand"
	"slices"
	"sync"
	"time"

	"raftkv/internal/bugs"
	"raftkv/internal/clock"
)

type Role uint8

const (
	Follower Role = iota
	Candidate
	Leader
)

func (r Role) String() string {
	return [...]string{"follower", "candidate", "leader"}[r]
}

type Config struct {
	ID    uint64
	Peers []uint64 // every member, including ID

	// ElectionTimeout is the minimum; each timer is random in [T, 2T).
	ElectionTimeout   time.Duration
	HeartbeatInterval time.Duration
	TickInterval      time.Duration
	RPCTimeout        time.Duration

	// LeaseDriftBound is the fraction of ElectionTimeout a lease gives up to
	// cover clock drift between nodes (thesis §6.4.1). 0.1 means the leader
	// assumes clocks drift by at most 10%.
	LeaseDriftBound float64

	// MaxBatch caps entries per AppendEntries and operations per fsync group.
	// 1 turns batching off.
	MaxBatch int
	// Pipeline lets the leader send further AppendEntries before earlier
	// ones are acknowledged, up to MaxInflight per follower.
	Pipeline    bool
	MaxInflight int

	Clock     clock.Clock
	Storage   Storage
	Transport Transport
	Apply     chan<- ApplyMsg
	Logger    *slog.Logger
}

func (c *Config) setDefaults() {
	if c.ElectionTimeout == 0 {
		c.ElectionTimeout = 300 * time.Millisecond
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = c.ElectionTimeout / 6
	}
	if c.TickInterval == 0 {
		c.TickInterval = 5 * time.Millisecond
	}
	if c.RPCTimeout == 0 {
		c.RPCTimeout = c.ElectionTimeout / 2
	}
	if c.MaxBatch == 0 {
		c.MaxBatch = 256
	}
	if c.MaxInflight == 0 {
		c.MaxInflight = 8
	}
	if !c.Pipeline {
		c.MaxInflight = 1
	}
	if c.Clock == nil {
		c.Clock = clock.NewSkewable()
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
}

// progress is the leader's view of one follower.
type progress struct {
	next, match uint64
	inflight    int
	snapSending bool
	ackSeq      uint64    // newest ReadIndex round the follower confirmed
	sentSeq     uint64    // newest ReadIndex round sent to the follower
	ackSent     time.Time // send time (leader clock) of the newest reply in this term
	active      bool      // replied since the last check-quorum
	notify      chan struct{}
}

type persistOp struct {
	seq       uint64
	hs        *HardState
	ents      []Entry
	snap      *Snapshot
	snapHS    HardState
	lastIndex uint64 // log last index once this op is applied
}

type Raft struct {
	cfg    Config
	id     uint64
	peers  []uint64
	quorum int
	log    *slog.Logger
	clock  clock.Clock

	mu sync.Mutex

	// persistent
	term uint64
	vote uint64
	// entries[0] is a sentinel holding the snapshot's index and term.
	entries []Entry
	snap    Snapshot

	// volatile
	role        Role
	leader      uint64
	commit      uint64
	lastApplied uint64
	pendingSnap *Snapshot
	votes       map[uint64]bool

	electionDeadline time.Time
	lastHeard        time.Time // last contact from a current leader
	bootTime         time.Time
	lastQuorumCheck  time.Time

	prs          map[uint64]*progress
	readSeq      uint64
	termStartSeq uint64 // persist seq of this term's leader no-op

	// persistence pipeline
	queue       []persistOp
	enqSeq      uint64
	durableSeq  uint64
	durableLast uint64
	persistCond *sync.Cond
	durableCond *sync.Cond

	applyCond *sync.Cond
	changeCh  chan struct{} // closed and replaced on every state change readers wait for

	stopped bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	rng     *rand.Rand
}

// New loads state from cfg.Storage and starts the node.
func New(cfg Config) (*Raft, error) {
	cfg.setDefaults()
	hs, snap, ents, err := cfg.Storage.Load()
	if err != nil {
		return nil, err
	}
	r := &Raft{
		cfg:      cfg,
		id:       cfg.ID,
		peers:    slices.Clone(cfg.Peers),
		quorum:   len(cfg.Peers)/2 + 1,
		clock:    cfg.Clock,
		term:     hs.Term,
		vote:     hs.Vote,
		snap:     snap,
		commit:   snap.Index,
		changeCh: make(chan struct{}),
		stopCh:   make(chan struct{}),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano() + int64(cfg.ID))),
	}
	r.log = cfg.Logger.With("node", cfg.ID)
	r.entries = append([]Entry{{Index: snap.Index, Term: snap.Term}}, ents...)
	r.durableLast = r.lastIndex()
	if snap.Index > 0 {
		s := snap
		r.pendingSnap = &s
	}
	r.persistCond = sync.NewCond(&r.mu)
	r.durableCond = sync.NewCond(&r.mu)
	r.applyCond = sync.NewCond(&r.mu)
	now := r.clock.Now()
	r.bootTime = now
	r.lastQuorumCheck = now
	r.resetElectionTimer(now)
	r.log.Info("raft started", "term", r.term, "vote", r.vote, "snapshot", snap.Index, "last_index", r.lastIndex(), "bugs", bugs.Active())

	r.wg.Add(3)
	go r.ticker()
	go r.persister()
	go r.applier()
	return r, nil
}

// Stop halts all goroutines. It does not close the storage.
func (r *Raft) Stop() {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	close(r.stopCh)
	r.persistCond.Broadcast()
	r.durableCond.Broadcast()
	r.applyCond.Broadcast()
	r.notifyLocked()
	r.mu.Unlock()
	r.wg.Wait()
}

// ---- log helpers (caller holds mu) ----

func (r *Raft) snapIndex() uint64 { return r.entries[0].Index }
func (r *Raft) lastIndex() uint64 { return r.entries[len(r.entries)-1].Index }
func (r *Raft) lastTerm() uint64  { return r.entries[len(r.entries)-1].Term }

func (r *Raft) termAt(i uint64) (uint64, bool) {
	if i < r.snapIndex() || i > r.lastIndex() {
		return 0, false
	}
	return r.entries[i-r.snapIndex()].Term, true
}

func (r *Raft) entryAt(i uint64) *Entry { return &r.entries[i-r.snapIndex()] }

// slice copies entries [lo, hi].
func (r *Raft) slice(lo, hi uint64) []Entry {
	if lo > hi {
		return nil
	}
	off := r.snapIndex()
	return slices.Clone(r.entries[lo-off : hi-off+1])
}

func (r *Raft) notifyLocked() {
	close(r.changeCh)
	r.changeCh = make(chan struct{})
}

// waitFor blocks until pred holds (checked under mu), ctx ends or the node stops.
func (r *Raft) waitFor(ctx context.Context, pred func() (bool, error)) error {
	for {
		r.mu.Lock()
		if r.stopped {
			r.mu.Unlock()
			return ErrStopped
		}
		ok, err := pred()
		ch := r.changeCh
		r.mu.Unlock()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return ErrTimeout
		}
	}
}

// ---- persistence ----

func (r *Raft) enqueue(op persistOp) uint64 {
	r.enqSeq++
	op.seq = r.enqSeq
	op.lastIndex = r.lastIndex()
	r.queue = append(r.queue, op)
	r.persistCond.Signal()
	return op.seq
}

func (r *Raft) persistHardState() uint64 {
	hs := HardState{Term: r.term, Vote: r.vote}
	return r.enqueue(persistOp{hs: &hs})
}

// waitDurable blocks (releasing mu) until op seq is on disk.
func (r *Raft) waitDurable(seq uint64) {
	for r.durableSeq < seq && !r.stopped {
		r.durableCond.Wait()
	}
}

func (r *Raft) persister() {
	defer r.wg.Done()
	var lastSync time.Time
	for {
		r.mu.Lock()
		for len(r.queue) == 0 && !r.stopped {
			r.persistCond.Wait()
		}
		if r.stopped {
			r.mu.Unlock()
			return
		}
		n := len(r.queue)
		if n > r.cfg.MaxBatch {
			n = r.cfg.MaxBatch
		}
		batch := slices.Clone(r.queue[:n])
		r.queue = slices.Delete(r.queue, 0, n)
		r.mu.Unlock()

		st := r.cfg.Storage
		var err error
		for _, op := range batch {
			switch {
			case op.snap != nil:
				err = st.SaveSnapshot(*op.snap, op.snapHS, op.ents)
			case op.hs != nil:
				err = st.SetHardState(*op.hs)
			default:
				err = st.Append(op.ents)
			}
			if err != nil {
				break
			}
		}
		if err == nil {
			if bugs.AckBeforeFsync {
				// BUG: treat written-but-unsynced data as durable and only
				// fsync in the background now and then.
				if time.Since(lastSync) > 500*time.Millisecond {
					lastSync = time.Now()
					go st.Sync()
				}
			} else {
				err = st.Sync()
			}
		}
		if err != nil {
			r.log.Error("storage failure, stopping", "err", err)
			go r.Stop()
			return
		}

		r.mu.Lock()
		last := batch[len(batch)-1]
		r.durableSeq = last.seq
		r.durableLast = last.lastIndex
		r.durableCond.Broadcast()
		if r.role == Leader {
			r.maybeCommit()
		}
		r.mu.Unlock()
	}
}

// ---- timers ----

func (r *Raft) resetElectionTimer(now time.Time) {
	t := r.cfg.ElectionTimeout
	r.electionDeadline = now.Add(t + time.Duration(r.rng.Int63n(int64(t))))
}

func (r *Raft) ticker() {
	defer r.wg.Done()
	tk := time.NewTicker(r.cfg.TickInterval)
	defer tk.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-tk.C:
		}
		now := r.clock.Now()
		r.mu.Lock()
		switch r.role {
		case Follower, Candidate:
			if now.After(r.electionDeadline) {
				r.startElection(now)
			}
		case Leader:
			if now.Sub(r.lastQuorumCheck) >= r.cfg.ElectionTimeout {
				r.checkQuorum(now)
			}
		}
		r.mu.Unlock()
	}
}

// checkQuorum steps down a leader that has not heard from a majority within
// an election timeout, so a partitioned leader stops accepting requests.
func (r *Raft) checkQuorum(now time.Time) {
	r.lastQuorumCheck = now
	active := 1
	for _, pr := range r.prs {
		if pr.active {
			active++
		}
		pr.active = false
	}
	if active < r.quorum {
		r.log.Info("lost quorum, stepping down", "term", r.term, "active", active)
		r.becomeFollower(r.term, None)
	}
}

// ---- roles ----

func (r *Raft) becomeFollower(term, leader uint64) {
	if term > r.term {
		r.term = term
		r.vote = None
		r.persistHardState()
	}
	if r.role != Follower {
		r.log.Info("became follower", "term", r.term, "leader", leader)
	}
	r.role = Follower
	r.leader = leader
	r.prs = nil
	r.notifyLocked()
}

func (r *Raft) startElection(now time.Time) {
	r.role = Candidate
	r.term++
	r.vote = r.id
	r.leader = None
	r.votes = map[uint64]bool{r.id: true}
	r.resetElectionTimer(now)
	seq := r.persistHardState()
	term := r.term
	req := &RequestVoteRequest{Term: term, Candidate: r.id, LastLogIndex: r.lastIndex(), LastLogTerm: r.lastTerm()}
	r.log.Info("starting election", "term", term, "last_index", req.LastLogIndex, "last_term", req.LastLogTerm)
	r.notifyLocked()

	go func() {
		r.mu.Lock()
		r.waitDurable(seq) // our own vote must be on disk before we ask for others
		if r.role != Candidate || r.term != term {
			r.mu.Unlock()
			return
		}
		if len(r.votes) >= r.quorum {
			r.becomeLeader()
		}
		r.mu.Unlock()
		for _, p := range r.peers {
			if p == r.id {
				continue
			}
			go r.requestVote(p, req)
		}
	}()
}

func (r *Raft) requestVote(peer uint64, req *RequestVoteRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.RPCTimeout)
	defer cancel()
	resp, err := r.cfg.Transport.RequestVote(ctx, peer, req)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if resp.Term > r.term {
		r.becomeFollower(resp.Term, None)
		return
	}
	if r.role != Candidate || r.term != req.Term || !resp.Granted {
		return
	}
	r.votes[peer] = true
	if len(r.votes) >= r.quorum {
		r.becomeLeader()
	}
}

func (r *Raft) becomeLeader() {
	r.role = Leader
	r.leader = r.id
	now := r.clock.Now()
	r.lastQuorumCheck = now
	r.prs = make(map[uint64]*progress)
	r.entries = append(r.entries, Entry{Index: r.lastIndex() + 1, Term: r.term, Type: EntryNoop})
	r.termStartSeq = r.enqueue(persistOp{ents: r.slice(r.lastIndex(), r.lastIndex())})
	r.log.Info("became leader", "term", r.term, "last_index", r.lastIndex())
	for _, p := range r.peers {
		if p == r.id {
			continue
		}
		pr := &progress{next: r.lastIndex(), notify: make(chan struct{}, 1), active: true}
		r.prs[p] = pr
		r.wg.Add(1)
		go r.replicator(p, r.term, pr)
	}
	r.maybeCommit()
	r.notifyLocked()
}

// selfMatch is how much of the leader's own log is durable.
func (r *Raft) selfMatch() uint64 {
	if bugs.AckBeforeFsync {
		return r.lastIndex()
	}
	if r.durableSeq < r.termStartSeq {
		return 0
	}
	return min(r.durableLast, r.lastIndex())
}

func (r *Raft) maybeCommit() {
	matches := []uint64{r.selfMatch()}
	for _, pr := range r.prs {
		matches = append(matches, pr.match)
	}
	slices.Sort(matches)
	n := matches[len(matches)-r.quorum]
	if n <= r.commit {
		return
	}
	if t, _ := r.termAt(n); t != r.term && !bugs.Figure8 {
		// Figure 8: an entry from an earlier term is only committed
		// indirectly, by committing one from the current term.
		return
	}
	r.commit = n
	r.applyCond.Signal()
	r.notifyLocked()
}

// ---- leader replication ----

func (r *Raft) wake(pr *progress) {
	select {
	case pr.notify <- struct{}{}:
	default:
	}
}

func (r *Raft) wakeAll() {
	for _, pr := range r.prs {
		r.wake(pr)
	}
}

func (r *Raft) replicator(peer, term uint64, pr *progress) {
	defer r.wg.Done()
	hb := time.NewTicker(r.cfg.HeartbeatInterval)
	defer hb.Stop()
	heartbeat := true
	for {
		r.mu.Lock()
		if r.stopped || r.role != Leader || r.term != term {
			r.mu.Unlock()
			return
		}
		r.sendLocked(peer, pr, heartbeat)
		r.mu.Unlock()
		heartbeat = false
		select {
		case <-r.stopCh:
			return
		case <-pr.notify:
		case <-hb.C:
			heartbeat = true
		}
	}
}

// sendLocked sends whatever the follower is missing, within the inflight
// window. An empty AppendEntries goes out as a heartbeat or read probe.
func (r *Raft) sendLocked(peer uint64, pr *progress, heartbeat bool) {
	for pr.inflight < r.cfg.MaxInflight {
		if pr.next <= r.snapIndex() {
			if !pr.snapSending {
				pr.snapSending = true
				pr.inflight++
				pr.sentSeq = r.readSeq
				req := &InstallSnapshotRequest{Term: r.term, Leader: r.id, ReadSeq: r.readSeq, Snapshot: r.snap}
				go r.sendSnapshot(peer, pr, req, r.clock.Now())
			}
			return
		}
		last := r.lastIndex()
		hi := min(last, pr.next+uint64(r.cfg.MaxBatch)-1)
		ents := r.slice(pr.next, hi)
		if len(ents) == 0 && !heartbeat && pr.sentSeq >= r.readSeq {
			return
		}
		pr.sentSeq = r.readSeq
		prevTerm, _ := r.termAt(pr.next - 1)
		req := &AppendEntriesRequest{
			Term: r.term, Leader: r.id,
			PrevIndex: pr.next - 1, PrevTerm: prevTerm,
			Commit: r.commit, ReadSeq: r.readSeq, Entries: ents,
		}
		if r.cfg.Pipeline && len(ents) > 0 {
			pr.next = hi + 1
		}
		pr.inflight++
		go r.sendAppend(peer, pr, req, r.clock.Now())
		if !r.cfg.Pipeline || len(ents) == 0 {
			return
		}
		heartbeat = false
	}
}

func (r *Raft) sendAppend(peer uint64, pr *progress, req *AppendEntriesRequest, sent time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.RPCTimeout)
	defer cancel()
	resp, err := r.cfg.Transport.AppendEntries(ctx, peer, req)

	r.mu.Lock()
	defer r.mu.Unlock()
	pr.inflight--
	defer r.wake(pr)
	if err != nil {
		if len(req.Entries) > 0 && pr.next > pr.match+1 && r.role == Leader && r.term == req.Term {
			pr.next = pr.match + 1 // resend what the lost message carried
		}
		return
	}
	if resp.Term > r.term {
		r.becomeFollower(resp.Term, None)
		return
	}
	if r.role != Leader || r.term != req.Term {
		return
	}
	r.recordAck(pr, req.ReadSeq, sent)
	if resp.Success {
		if resp.MatchIndex > pr.match {
			pr.match = resp.MatchIndex
		}
		if pr.next < pr.match+1 {
			pr.next = pr.match + 1
		}
		r.maybeCommit()
		return
	}
	// Log mismatch: back up using the follower's conflict hint (§5.3).
	next := resp.ConflictIndex
	if resp.ConflictTerm != 0 {
		for i := r.lastIndex(); i > r.snapIndex(); i-- {
			if t, _ := r.termAt(i); t == resp.ConflictTerm {
				next = i + 1
				break
			} else if t < resp.ConflictTerm {
				break
			}
		}
	}
	next = max(next, pr.match+1)
	next = min(next, r.lastIndex()+1)
	if next < pr.next || !r.cfg.Pipeline {
		pr.next = next
	}
	r.log.Debug("append rejected", "term", r.term, "peer", peer, "prev_index", req.PrevIndex, "next", pr.next)
}

func (r *Raft) sendSnapshot(peer uint64, pr *progress, req *InstallSnapshotRequest, sent time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*r.cfg.RPCTimeout)
	defer cancel()
	resp, err := r.cfg.Transport.InstallSnapshot(ctx, peer, req)
	r.mu.Lock()
	defer r.mu.Unlock()
	pr.inflight--
	pr.snapSending = false
	defer r.wake(pr)
	if err != nil {
		return
	}
	if resp.Term > r.term {
		r.becomeFollower(resp.Term, None)
		return
	}
	if r.role != Leader || r.term != req.Term {
		return
	}
	r.recordAck(pr, req.ReadSeq, sent)
	pr.match = max(pr.match, req.Snapshot.Index)
	pr.next = max(pr.next, pr.match+1)
	r.maybeCommit()
}

// recordAck notes that the follower accepted us as leader for a message sent
// at time sent (leader clock), which confirms reads and extends the lease.
func (r *Raft) recordAck(pr *progress, readSeq uint64, sent time.Time) {
	pr.active = true
	if readSeq > pr.ackSeq {
		pr.ackSeq = readSeq
	}
	if sent.After(pr.ackSent) {
		pr.ackSent = sent
	}
	r.notifyLocked()
}

// ---- RPC handlers ----

// inLease reports whether this node recently heard from a live leader (or is
// one with an active quorum). Such nodes refuse votes: this keeps a partitioned
// candidate from disrupting a healthy cluster, and is what makes leader leases
// safe (thesis §6.4.1).
func (r *Raft) inLease(now time.Time) bool {
	t := r.cfg.ElectionTimeout
	if now.Sub(r.bootTime) < t {
		// A restarted node may have acknowledged a lease just before it
		// crashed, so it waits one timeout before voting.
		return true
	}
	switch r.role {
	case Leader:
		return now.Sub(r.quorumAckTime(now)) < t
	case Follower:
		return r.leader != None && now.Sub(r.lastHeard) < t
	}
	return false
}

// quorumAckTime is the newest time such that a majority (counting the leader)
// has acknowledged a message sent at or after it.
func (r *Raft) quorumAckTime(now time.Time) time.Time {
	times := []time.Time{now}
	for _, pr := range r.prs {
		times = append(times, pr.ackSent)
	}
	slices.SortFunc(times, func(a, b time.Time) int { return b.Compare(a) })
	return times[r.quorum-1]
}

func (r *Raft) HandleRequestVote(req *RequestVoteRequest) *RequestVoteResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()
	if req.Term > r.term && r.inLease(now) {
		return &RequestVoteResponse{Term: r.term}
	}
	if req.Term > r.term {
		r.becomeFollower(req.Term, None)
	}
	resp := &RequestVoteResponse{Term: r.term}
	if req.Term < r.term {
		return resp
	}
	upToDate := req.LastLogTerm > r.lastTerm() ||
		(req.LastLogTerm == r.lastTerm() && req.LastLogIndex >= r.lastIndex())
	if (r.vote == None || r.vote == req.Candidate) && upToDate {
		r.vote = req.Candidate
		r.resetElectionTimer(now)
		if bugs.VoteNoPersist {
			// BUG: answer from memory; votedFor is lost if we crash now.
			hs := HardState{Term: r.term, Vote: None}
			r.waitDurable(r.enqueue(persistOp{hs: &hs}))
		} else {
			r.waitDurable(r.persistHardState())
		}
		if r.term != req.Term || r.vote != req.Candidate {
			return &RequestVoteResponse{Term: r.term}
		}
		resp.Granted = true
		r.log.Debug("granted vote", "term", r.term, "candidate", req.Candidate)
	}
	return resp
}

// acceptLeader handles the term check shared by AppendEntries and
// InstallSnapshot. It returns false if the request is stale.
func (r *Raft) acceptLeader(term, leader uint64) bool {
	if term < r.term {
		return false
	}
	if term > r.term || r.role != Follower || r.leader != leader {
		r.becomeFollower(term, leader)
	}
	now := r.clock.Now()
	r.lastHeard = now
	r.resetElectionTimer(now)
	return true
}

func (r *Raft) HandleAppendEntries(req *AppendEntriesRequest) *AppendEntriesResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.acceptLeader(req.Term, req.Leader) {
		return &AppendEntriesResponse{Term: r.term}
	}
	resp := &AppendEntriesResponse{Term: r.term}
	prev, prevTerm, ents := req.PrevIndex, req.PrevTerm, req.Entries

	// Entries already folded into our snapshot are committed, so they match.
	if prev < r.snapIndex() {
		skip := r.snapIndex() - prev
		if skip >= uint64(len(ents)) {
			resp.Success = true
			resp.MatchIndex = prev + uint64(len(ents))
			return resp
		}
		ents = ents[skip:]
		prev, prevTerm = r.snapIndex(), r.entries[0].Term
	}
	if prev > r.lastIndex() {
		resp.ConflictIndex = r.lastIndex() + 1
		return resp
	}
	if t, _ := r.termAt(prev); t != prevTerm {
		resp.ConflictTerm = t
		i := prev
		for i > r.snapIndex()+1 {
			if pt, _ := r.termAt(i - 1); pt != t {
				break
			}
			i--
		}
		resp.ConflictIndex = i
		return resp
	}

	var changed []Entry
	for i, e := range ents {
		if e.Index > r.lastIndex() {
			r.entries = append(r.entries, ents[i:]...)
			changed = ents[i:]
			break
		}
		if t, _ := r.termAt(e.Index); t != e.Term {
			if bugs.NoTruncate {
				// BUG: overwrite the overlapping entries but keep any
				// conflicting suffix beyond them.
				for _, ne := range ents[i:] {
					if ne.Index <= r.lastIndex() {
						*r.entryAt(ne.Index) = ne
					} else {
						r.entries = append(r.entries, ne)
					}
				}
				changed = r.slice(e.Index, r.lastIndex())
			} else {
				r.entries = append(r.entries[:e.Index-r.snapIndex()], ents[i:]...)
				changed = ents[i:]
			}
			r.log.Info("truncated conflicting suffix", "term", r.term, "index", e.Index)
			break
		}
	}
	newLast := prev + uint64(len(ents))
	if req.Commit > r.commit {
		c := min(req.Commit, newLast)
		if c > r.commit {
			r.commit = c
			r.applyCond.Signal()
			r.notifyLocked()
		}
	}
	if len(changed) > 0 {
		r.enqueue(persistOp{ents: slices.Clone(changed)})
	}
	// Figure 2: respond only after stable storage is updated. The entries
	// up to newLast may still be queued by an earlier request (this one can
	// be a retransmission, or a heartbeat pointing past them), so wait for
	// every write queued so far, not just this request's own.
	if seq := r.enqSeq; r.durableSeq < seq {
		term := r.term
		r.waitDurable(seq)
		if r.term != term || r.durableSeq < seq {
			return &AppendEntriesResponse{Term: r.term}
		}
	}
	resp.Success = true
	resp.MatchIndex = newLast
	return resp
}

func (r *Raft) HandleInstallSnapshot(req *InstallSnapshotRequest) *InstallSnapshotResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.acceptLeader(req.Term, req.Leader) {
		return &InstallSnapshotResponse{Term: r.term}
	}
	s := req.Snapshot
	if s.Index <= r.commit || s.Index <= r.lastApplied {
		return &InstallSnapshotResponse{Term: r.term}
	}
	var rest []Entry
	if t, ok := r.termAt(s.Index); ok && t == s.Term {
		rest = r.slice(s.Index+1, r.lastIndex())
	}
	r.entries = append([]Entry{{Index: s.Index, Term: s.Term}}, rest...)
	r.snap = s
	r.commit = s.Index
	r.pendingSnap = &s
	r.applyCond.Signal()
	r.log.Info("installing snapshot from leader", "term", r.term, "index", s.Index)
	seq := r.enqueue(persistOp{snap: &s, snapHS: HardState{Term: r.term, Vote: r.vote}, ents: slices.Clone(rest)})
	r.waitDurable(seq)
	return &InstallSnapshotResponse{Term: r.term}
}

// ---- applier ----

func (r *Raft) applier() {
	defer r.wg.Done()
	for {
		r.mu.Lock()
		for !r.stopped && r.pendingSnap == nil && r.lastApplied >= r.commit {
			r.applyCond.Wait()
		}
		if r.stopped {
			r.mu.Unlock()
			return
		}
		var msgs []ApplyMsg
		if s := r.pendingSnap; s != nil {
			r.pendingSnap = nil
			if s.Index > r.lastApplied {
				msgs = append(msgs, ApplyMsg{Snapshot: s})
				r.lastApplied = s.Index
			}
		} else {
			lo := max(r.lastApplied+1, r.snapIndex()+1)
			for _, e := range r.slice(lo, r.commit) {
				e := e
				msgs = append(msgs, ApplyMsg{Entry: &e})
			}
			r.lastApplied = r.commit
		}
		r.mu.Unlock()
		for _, m := range msgs {
			select {
			case r.cfg.Apply <- m:
			case <-r.stopCh:
				return
			}
		}
	}
}

// ---- public API ----

// Propose appends data to the leader's log. The entry is committed once it
// shows up on the apply channel at the returned index with the returned term.
func (r *Raft) Propose(data []byte) (index, term uint64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return 0, 0, ErrStopped
	}
	if r.role != Leader {
		return 0, 0, ErrNotLeader
	}
	e := Entry{Index: r.lastIndex() + 1, Term: r.term, Type: EntryNormal, Data: data}
	r.entries = append(r.entries, e)
	r.enqueue(persistOp{ents: []Entry{e}})
	r.wakeAll()
	return e.Index, e.Term, nil
}

// ReadIndex implements the leader side of the ReadIndex protocol (thesis
// §6.4): it returns an index such that applying the log up to it and then
// reading the state machine yields a linearizable read.
func (r *Raft) ReadIndex(ctx context.Context) (uint64, error) {
	var readIdx, seq, term uint64
	// The leader must have committed an entry from its own term first.
	err := r.waitFor(ctx, func() (bool, error) {
		if r.role != Leader {
			return false, ErrNotLeader
		}
		t, _ := r.termAt(r.commit)
		return t == r.term, nil
	})
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	if r.role != Leader {
		r.mu.Unlock()
		return 0, ErrNotLeader
	}
	readIdx = r.commit
	r.readSeq++
	seq, term = r.readSeq, r.term
	for p, pr := range r.prs {
		r.sendLocked(p, pr, false) // sends a probe unless the window is full
	}
	r.mu.Unlock()

	err = r.waitFor(ctx, func() (bool, error) {
		if r.role != Leader || r.term != term {
			return false, ErrNotLeader
		}
		acks := 1
		for _, pr := range r.prs {
			if pr.ackSeq >= seq {
				acks++
			}
		}
		return acks >= r.quorum, nil
	})
	if err != nil {
		return 0, err
	}
	return readIdx, nil
}

// LeaseRead returns the commit index if this leader's lease is still valid,
// meaning no other leader can have been elected since a majority last
// acknowledged it. ok is false if the caller must fall back to ReadIndex.
func (r *Raft) LeaseRead() (index uint64, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.role != Leader {
		return 0, false
	}
	if t, _ := r.termAt(r.commit); t != r.term {
		return 0, false
	}
	now := r.clock.Now()
	lease := r.cfg.ElectionTimeout
	if !bugs.LeaseNoDrift { // BUG when off: trust the full timeout despite drift
		lease = time.Duration(float64(lease) * (1 - r.cfg.LeaseDriftBound))
	}
	if now.Before(r.quorumAckTime(now).Add(lease)) {
		return r.commit, true
	}
	return 0, false
}

// Commit returns the current commit index.
func (r *Raft) Commit() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.commit
}

// Snapshot tells Raft the state machine has captured everything up to index
// in data, so the log prefix can be discarded.
func (r *Raft) Snapshot(index uint64, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index <= r.snapIndex() || index > r.lastApplied {
		return
	}
	t, _ := r.termAt(index)
	r.snap = Snapshot{Index: index, Term: t, Data: data}
	rest := r.slice(index+1, r.lastIndex())
	r.entries = append([]Entry{{Index: index, Term: t}}, rest...)
	s := r.snap
	r.enqueue(persistOp{snap: &s, snapHS: HardState{Term: r.term, Vote: r.vote}, ents: rest})
	r.log.Debug("compacted log", "term", r.term, "index", index)
}

type Status struct {
	ID, Term, Leader, Commit, Applied, LastIndex, SnapIndex uint64
	Role                                                    Role
}

func (r *Raft) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Status{
		ID: r.id, Term: r.term, Leader: r.leader, Commit: r.commit, Applied: r.lastApplied,
		LastIndex: r.lastIndex(), SnapIndex: r.snapIndex(), Role: r.role,
	}
}

// StorageSize reports the log size so the state machine can decide to snapshot.
func (r *Raft) StorageSize() int64 { return r.cfg.Storage.Size() }
