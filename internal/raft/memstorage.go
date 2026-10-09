package raft

import (
	"slices"
	"sync"
)

// MemStorage is an in-memory Storage for tests. Writes survive a simulated
// process crash unless Lazy is set, in which case anything written since the
// last Sync is lost (the analogue of wal.Options.Lazy).
type MemStorage struct {
	mu      sync.Mutex
	Lazy    bool
	cur     memState
	durable memState
}

type memState struct {
	hs   HardState
	snap Snapshot
	ents []Entry
	size int64
}

func (s memState) clone() memState {
	s.ents = slices.Clone(s.ents)
	return s
}

func NewMemStorage() *MemStorage { return &MemStorage{} }

// Crash discards unsynced writes in Lazy mode. Call it before reusing the
// storage for a restarted node.
func (m *MemStorage) Crash() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Lazy {
		m.cur = m.durable.clone()
	}
}

func (m *MemStorage) Load() (HardState, Snapshot, []Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cur.hs, m.cur.snap, slices.Clone(m.cur.ents), nil
}

func (m *MemStorage) SetHardState(hs HardState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cur.hs = hs
	m.cur.size += 16
	return nil
}

func (m *MemStorage) Append(ents []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(ents) == 0 {
		return nil
	}
	first := ents[0].Index
	for len(m.cur.ents) > 0 && m.cur.ents[len(m.cur.ents)-1].Index >= first {
		m.cur.ents = m.cur.ents[:len(m.cur.ents)-1]
	}
	for _, e := range ents {
		m.cur.size += int64(len(e.Data) + 17)
	}
	m.cur.ents = append(m.cur.ents, ents...)
	return nil
}

func (m *MemStorage) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durable = m.cur.clone()
	return nil
}

func (m *MemStorage) SaveSnapshot(snap Snapshot, hs HardState, ents []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cur = memState{hs: hs, snap: snap, ents: slices.Clone(ents)}
	for _, e := range ents {
		m.cur.size += int64(len(e.Data) + 17)
	}
	m.durable = m.cur.clone()
	return nil
}

func (m *MemStorage) Size() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cur.size
}

func (m *MemStorage) Close() error { return nil }
