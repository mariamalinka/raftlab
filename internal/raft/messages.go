package raft

import "context"

type RequestVoteRequest struct {
	Term         uint64
	Candidate    uint64
	LastLogIndex uint64
	LastLogTerm  uint64
}

type RequestVoteResponse struct {
	Term    uint64
	Granted bool
}

type AppendEntriesRequest struct {
	Term      uint64
	Leader    uint64
	PrevIndex uint64
	PrevTerm  uint64
	Commit    uint64
	// ReadSeq is the newest ReadIndex round at send time. Any reply in the
	// same term confirms leadership for every round up to it.
	ReadSeq uint64
	Entries []Entry
}

type AppendEntriesResponse struct {
	Term       uint64
	Success    bool
	MatchIndex uint64
	// On a log mismatch: the follower's conflicting term (0 if its log is
	// too short) and the first index it holds for that term.
	ConflictIndex uint64
	ConflictTerm  uint64
}

type InstallSnapshotRequest struct {
	Term     uint64
	Leader   uint64
	ReadSeq  uint64
	Snapshot Snapshot
}

type InstallSnapshotResponse struct {
	Term uint64
}

// Transport carries RPCs between nodes. The gRPC transport implements it;
// tests use an in-memory network that can drop, delay and partition.
type Transport interface {
	RequestVote(ctx context.Context, to uint64, req *RequestVoteRequest) (*RequestVoteResponse, error)
	AppendEntries(ctx context.Context, to uint64, req *AppendEntriesRequest) (*AppendEntriesResponse, error)
	InstallSnapshot(ctx context.Context, to uint64, req *InstallSnapshotRequest) (*InstallSnapshotResponse, error)
}

// ApplyMsg is delivered in log order on the apply channel: either one
// committed entry or a snapshot that replaces the state machine.
type ApplyMsg struct {
	Entry    *Entry
	Snapshot *Snapshot
}
