// Package kv is the replicated key-value state machine on top of Raft.
//
// Writes (Put, CAS) go through the log and are deduplicated per client
// session, so a retried request is applied at most once. Reads take one of the
// three paths compared in experiment E5:
//
//   - log:       the Get is appended to the log like a write (slowest, simplest)
//   - readindex: the leader confirms leadership with a heartbeat round, then
//     reads locally once it has applied up to the commit index it saw
//   - lease:     the leader skips the heartbeat round while its lease holds,
//     falling back to ReadIndex otherwise; safe only under bounded clock drift
package kv

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"raftkv/internal/bugs"
	"raftkv/internal/raft"
)

type ReadMode string

const (
	ReadLog   ReadMode = "log"
	ReadIndex ReadMode = "readindex"
	ReadLease ReadMode = "lease"
)

func ParseReadMode(s string) (ReadMode, error) {
	switch m := ReadMode(s); m {
	case ReadLog, ReadIndex, ReadLease:
		return m, nil
	}
	return "", fmt.Errorf("unknown read mode %q (log, readindex, lease)", s)
}

var (
	ErrNotLeader = errors.New("kv: not leader")
	ErrTimeout   = errors.New("kv: timeout")
	ErrStopped   = errors.New("kv: stopped")
)

type opType uint8

const (
	opGet opType = iota + 1
	opPut
	opCAS
)

type command struct {
	op            opType
	clientID, seq uint64
	key           string
	value         string
	expected      string
}

func (c command) encode() []byte {
	b := []byte{byte(c.op)}
	b = binary.AppendUvarint(b, c.clientID)
	b = binary.AppendUvarint(b, c.seq)
	for _, s := range []string{c.key, c.value, c.expected} {
		b = binary.AppendUvarint(b, uint64(len(s)))
		b = append(b, s...)
	}
	return b
}

func decodeCommand(b []byte) (command, error) {
	var c command
	if len(b) == 0 {
		return c, errors.New("empty command")
	}
	c.op = opType(b[0])
	b = b[1:]
	next := func() uint64 {
		v, n := binary.Uvarint(b)
		if n <= 0 {
			return 0
		}
		b = b[n:]
		return v
	}
	c.clientID, c.seq = next(), next()
	strs := make([]string, 3)
	for i := range strs {
		n := next()
		if n > uint64(len(b)) {
			return c, errors.New("short command")
		}
		strs[i], b = string(b[:n]), b[n:]
	}
	c.key, c.value, c.expected = strs[0], strs[1], strs[2]
	return c, nil
}

// Result of applying one command.
type Result struct {
	Value string
	Found bool
	OK    bool // CAS succeeded
}

type session struct {
	Seq    uint64
	Result Result
}

type waiter struct {
	term uint64
	ch   chan waitResult
}

type waitResult struct {
	res Result
	err error
}

type Config struct {
	Raft     raft.Config // Apply is filled in by New
	ReadMode ReadMode
	// SnapshotBytes triggers a snapshot once the log exceeds this size.
	// 0 disables snapshots.
	SnapshotBytes int64
	Logger        *slog.Logger
}

type Store struct {
	cfg Config
	rf  *raft.Raft
	log *slog.Logger

	mu       sync.Mutex
	data     map[string]string
	sessions map[uint64]session
	applied  uint64
	lastSnap uint64
	waiters  map[uint64][]waiter
	changed  chan struct{} // closed when applied advances
	stopCh   chan struct{}
	done     chan struct{}
}

func New(cfg Config) (*Store, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.ReadMode == "" {
		cfg.ReadMode = ReadIndex
	}
	apply := make(chan raft.ApplyMsg, 1024)
	cfg.Raft.Apply = apply
	cfg.Raft.Logger = cfg.Logger
	s := &Store{
		cfg:      cfg,
		log:      cfg.Logger.With("node", cfg.Raft.ID),
		data:     map[string]string{},
		sessions: map[uint64]session{},
		waiters:  map[uint64][]waiter{},
		changed:  make(chan struct{}),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
	rf, err := raft.New(cfg.Raft)
	if err != nil {
		return nil, err
	}
	s.rf = rf
	go s.applyLoop(apply)
	return s, nil
}

func (s *Store) Raft() *raft.Raft { return s.rf }

func (s *Store) Stop() {
	close(s.stopCh)
	s.rf.Stop()
	<-s.done
}

func (s *Store) applyLoop(apply <-chan raft.ApplyMsg) {
	defer close(s.done)
	for {
		var m raft.ApplyMsg
		select {
		case m = <-apply:
		case <-s.stopCh:
			return
		}
		s.mu.Lock()
		if m.Snapshot != nil {
			s.restore(m.Snapshot)
		} else {
			s.applyEntry(m.Entry)
		}
		close(s.changed)
		s.changed = make(chan struct{})
		snapIndex := s.applied
		var snap []byte
		if s.cfg.SnapshotBytes > 0 && s.applied >= s.lastSnap+100 && s.rf.StorageSize() > s.cfg.SnapshotBytes {
			snap = s.encodeSnapshot()
			s.lastSnap = s.applied
		}
		s.mu.Unlock()
		if snap != nil {
			s.rf.Snapshot(snapIndex, snap)
		}
	}
}

func (s *Store) applyEntry(e *raft.Entry) {
	if e.Index != s.applied+1 {
		s.log.Error("apply out of order", "index", e.Index, "applied", s.applied)
	}
	s.applied = e.Index
	var res Result
	if e.Type == raft.EntryNormal {
		c, err := decodeCommand(e.Data)
		if err != nil {
			s.log.Error("bad command", "index", e.Index, "err", err)
		} else {
			res = s.execute(c)
		}
	}
	for _, w := range s.waiters[e.Index] {
		if w.term == e.Term {
			w.ch <- waitResult{res: res}
		} else {
			w.ch <- waitResult{err: ErrNotLeader} // our entry was overwritten
		}
	}
	delete(s.waiters, e.Index)
}

func (s *Store) execute(c command) Result {
	if c.op == opGet {
		v, ok := s.data[c.key]
		return Result{Value: v, Found: ok}
	}
	if !bugs.NoDedup { // BUG when skipped: a retried write is applied again
		if sess, ok := s.sessions[c.clientID]; ok && c.seq <= sess.Seq {
			return sess.Result
		}
	}
	var res Result
	switch c.op {
	case opPut:
		s.data[c.key] = c.value
		res = Result{OK: true}
	case opCAS:
		if s.data[c.key] == c.expected {
			s.data[c.key] = c.value
			res = Result{OK: true}
		}
	}
	s.sessions[c.clientID] = session{Seq: c.seq, Result: res}
	return res
}

type snapshotData struct {
	Data     map[string]string
	Sessions map[uint64]session
}

func (s *Store) encodeSnapshot() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(snapshotData{Data: s.data, Sessions: s.sessions}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (s *Store) restore(snap *raft.Snapshot) {
	var d snapshotData
	if err := gob.NewDecoder(bytes.NewReader(snap.Data)).Decode(&d); err != nil {
		s.log.Error("bad snapshot", "index", snap.Index, "err", err)
		return
	}
	if d.Data == nil {
		d.Data = map[string]string{}
	}
	if d.Sessions == nil {
		d.Sessions = map[uint64]session{}
	}
	s.data, s.sessions = d.Data, d.Sessions
	s.applied = snap.Index
	// Anyone waiting on an index the snapshot skipped over lost their entry
	// or will learn the outcome by retrying (sessions make that safe).
	for idx, ws := range s.waiters {
		if idx <= snap.Index {
			for _, w := range ws {
				w.ch <- waitResult{err: ErrNotLeader}
			}
			delete(s.waiters, idx)
		}
	}
}

// propose appends c and waits until it is applied.
func (s *Store) propose(ctx context.Context, c command) (Result, error) {
	ch := make(chan waitResult, 1)
	s.mu.Lock()
	idx, term, err := s.rf.Propose(c.encode())
	if err != nil {
		s.mu.Unlock()
		return Result{}, mapErr(err)
	}
	s.waiters[idx] = append(s.waiters[idx], waiter{term: term, ch: ch})
	s.mu.Unlock()
	select {
	case r := <-ch:
		return r.res, r.err
	case <-ctx.Done():
		return Result{}, ErrTimeout
	case <-s.stopCh:
		return Result{}, ErrStopped
	}
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, raft.ErrNotLeader):
		return ErrNotLeader
	case errors.Is(err, raft.ErrTimeout):
		return ErrTimeout
	case errors.Is(err, raft.ErrStopped):
		return ErrStopped
	}
	return err
}

// waitApplied blocks until the state machine has applied index.
func (s *Store) waitApplied(ctx context.Context, index uint64) error {
	for {
		s.mu.Lock()
		if s.applied >= index {
			s.mu.Unlock()
			return nil
		}
		ch := s.changed
		s.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ErrTimeout
		case <-s.stopCh:
			return ErrStopped
		}
	}
}

func (s *Store) readLocal(key string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return Result{Value: v, Found: ok}
}

func (s *Store) Get(ctx context.Context, key string) (Result, error) {
	if bugs.LocalRead {
		// BUG: a node that believes it is leader answers from local state,
		// even if a newer leader has since been elected elsewhere.
		if s.rf.Status().Role == raft.Leader {
			return s.readLocal(key), nil
		}
		return Result{}, ErrNotLeader
	}
	switch s.cfg.ReadMode {
	case ReadLog:
		return s.propose(ctx, command{op: opGet, key: key})
	case ReadLease:
		if idx, ok := s.rf.LeaseRead(); ok {
			if err := s.waitApplied(ctx, idx); err != nil {
				return Result{}, err
			}
			return s.readLocal(key), nil
		}
	}
	idx, err := s.rf.ReadIndex(ctx)
	if err != nil {
		return Result{}, mapErr(err)
	}
	if err := s.waitApplied(ctx, idx); err != nil {
		return Result{}, err
	}
	return s.readLocal(key), nil
}

func (s *Store) Put(ctx context.Context, clientID, seq uint64, key, value string) error {
	_, err := s.propose(ctx, command{op: opPut, clientID: clientID, seq: seq, key: key, value: value})
	return err
}

// CAS sets key to value if its current value is expected ("" for a missing
// key). ok reports whether the swap happened.
func (s *Store) CAS(ctx context.Context, clientID, seq uint64, key, expected, value string) (bool, error) {
	r, err := s.propose(ctx, command{op: opCAS, clientID: clientID, seq: seq, key: key, expected: expected, value: value})
	return r.OK, err
}
