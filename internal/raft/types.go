package raft

import "errors"

// None is the vote value meaning "voted for nobody". Node IDs start at 1.
const None uint64 = 0

type EntryType uint8

const (
	EntryNormal EntryType = iota
	// EntryNoop is appended by every new leader so it can commit an entry in
	// its own term (needed before serving ReadIndex or lease reads).
	EntryNoop
)

type Entry struct {
	Index uint64
	Term  uint64
	Type  EntryType
	Data  []byte
}

// HardState is the persistent per-node state from Figure 2 of the Raft paper.
type HardState struct {
	Term uint64
	Vote uint64
}

type Snapshot struct {
	Index uint64
	Term  uint64
	Data  []byte
}

// Storage is the durable log. The write-ahead log in package wal implements
// it; tests use MemStorage.
type Storage interface {
	// Load returns everything that survived the last crash.
	Load() (HardState, Snapshot, []Entry, error)
	SetHardState(hs HardState) error
	// Append writes entries. If ents[0].Index is not past the current last
	// index, the stored suffix from that index on is replaced.
	Append(ents []Entry) error
	// Sync makes every earlier write durable.
	Sync() error
	// SaveSnapshot durably installs snap and rewrites the log as hs + ents,
	// where every entry has Index > snap.Index.
	SaveSnapshot(snap Snapshot, hs HardState, ents []Entry) error
	// Size is the log size in bytes, used to decide when to snapshot.
	Size() int64
	Close() error
}

var (
	ErrNotLeader = errors.New("raft: not leader")
	ErrStopped   = errors.New("raft: stopped")
	ErrTimeout   = errors.New("raft: timeout")
)
