// Package rpc carries Raft and client traffic over gRPC.
//
// Messages are plain Go structs encoded with a small hand-written binary
// codec registered under the gRPC content subtype "rkv", so no protoc step is
// needed. The codec cost is part of what E3/E7 measure against etcd's
// protobuf encoding; it is noted in the threats-to-validity list.
package rpc

import (
	"encoding/binary"
	"errors"
	"fmt"

	"google.golang.org/grpc/encoding"

	"raftkv/internal/raft"
)

const codecName = "rkv"

func init() { encoding.RegisterCodec(codec{}) }

// ---- client API messages ----

type GetRequest struct{ Key string }
type GetResponse struct {
	Value  string
	Found  bool
	Err    string
	Leader uint64 // hint when Err == ErrNotLeader
}

type PutRequest struct {
	ClientID, Seq uint64
	Key, Value    string
}
type PutResponse struct {
	Err    string
	Leader uint64
}

type CASRequest struct {
	ClientID, Seq   uint64
	Key             string
	Expected, Value string
}
type CASResponse struct {
	OK     bool // false: the current value was not Expected
	Err    string
	Leader uint64
}

type StatusRequest struct{}
type StatusResponse struct {
	ID, Term, Leader, Commit, Applied, LastIndex, SnapIndex uint64
	Role                                                    string
	ClockOffsetNanos                                        int64
	ClockRate                                               float64
	Bugs                                                    []string
}

// ClockRequest skews the node's clock: Jump moves it, Rate (if > 0) sets
// its speed, Reset restores real time.
type ClockRequest struct {
	JumpNanos int64
	Rate      float64
	Reset     bool
}
type ClockResponse struct{}

const (
	ErrNotLeader = "not_leader"
	ErrTimeout   = "timeout"
	ErrStopped   = "stopped"
)

// ---- codec ----

type codec struct{}

func (codec) Name() string { return codecName }

func (codec) Marshal(v any) ([]byte, error) {
	var e enc
	switch m := v.(type) {
	case *raft.RequestVoteRequest:
		e.u(m.Term, m.Candidate, m.LastLogIndex, m.LastLogTerm)
	case *raft.RequestVoteResponse:
		e.u(m.Term)
		e.bool(m.Granted)
	case *raft.AppendEntriesRequest:
		e.u(m.Term, m.Leader, m.PrevIndex, m.PrevTerm, m.Commit, m.ReadSeq, uint64(len(m.Entries)))
		for _, x := range m.Entries {
			e.u(x.Index, x.Term, uint64(x.Type))
			e.bytes(x.Data)
		}
	case *raft.AppendEntriesResponse:
		e.u(m.Term)
		e.bool(m.Success)
		e.u(m.MatchIndex, m.ConflictIndex, m.ConflictTerm)
	case *raft.InstallSnapshotRequest:
		e.u(m.Term, m.Leader, m.ReadSeq, m.Snapshot.Index, m.Snapshot.Term)
		e.bytes(m.Snapshot.Data)
	case *raft.InstallSnapshotResponse:
		e.u(m.Term)
	case *GetRequest:
		e.str(m.Key)
	case *GetResponse:
		e.str(m.Value)
		e.bool(m.Found)
		e.str(m.Err)
		e.u(m.Leader)
	case *PutRequest:
		e.u(m.ClientID, m.Seq)
		e.str(m.Key)
		e.str(m.Value)
	case *PutResponse:
		e.str(m.Err)
		e.u(m.Leader)
	case *CASRequest:
		e.u(m.ClientID, m.Seq)
		e.str(m.Key)
		e.str(m.Expected)
		e.str(m.Value)
	case *CASResponse:
		e.bool(m.OK)
		e.str(m.Err)
		e.u(m.Leader)
	case *StatusRequest, *ClockResponse:
	case *StatusResponse:
		e.u(m.ID, m.Term, m.Leader, m.Commit, m.Applied, m.LastIndex, m.SnapIndex)
		e.str(m.Role)
		e.u(uint64(m.ClockOffsetNanos))
		e.str(fmt.Sprint(m.ClockRate))
		e.u(uint64(len(m.Bugs)))
		for _, b := range m.Bugs {
			e.str(b)
		}
	case *ClockRequest:
		e.u(uint64(m.JumpNanos))
		e.str(fmt.Sprint(m.Rate))
		e.bool(m.Reset)
	default:
		return nil, fmt.Errorf("rkv codec: cannot marshal %T", v)
	}
	return e.b, nil
}

func (codec) Unmarshal(data []byte, v any) error {
	d := dec{b: data}
	switch m := v.(type) {
	case *raft.RequestVoteRequest:
		m.Term, m.Candidate, m.LastLogIndex, m.LastLogTerm = d.u(), d.u(), d.u(), d.u()
	case *raft.RequestVoteResponse:
		m.Term, m.Granted = d.u(), d.bool()
	case *raft.AppendEntriesRequest:
		m.Term, m.Leader, m.PrevIndex, m.PrevTerm, m.Commit, m.ReadSeq = d.u(), d.u(), d.u(), d.u(), d.u(), d.u()
		n := d.u()
		if n > uint64(len(data)) {
			return errors.New("rkv codec: bad entry count")
		}
		m.Entries = make([]raft.Entry, n)
		for i := range m.Entries {
			x := &m.Entries[i]
			x.Index, x.Term, x.Type = d.u(), d.u(), raft.EntryType(d.u())
			x.Data = d.bytes()
		}
	case *raft.AppendEntriesResponse:
		m.Term, m.Success = d.u(), d.bool()
		m.MatchIndex, m.ConflictIndex, m.ConflictTerm = d.u(), d.u(), d.u()
	case *raft.InstallSnapshotRequest:
		m.Term, m.Leader, m.ReadSeq, m.Snapshot.Index, m.Snapshot.Term = d.u(), d.u(), d.u(), d.u(), d.u()
		m.Snapshot.Data = d.bytes()
	case *raft.InstallSnapshotResponse:
		m.Term = d.u()
	case *GetRequest:
		m.Key = d.str()
	case *GetResponse:
		m.Value, m.Found, m.Err, m.Leader = d.str(), d.bool(), d.str(), d.u()
	case *PutRequest:
		m.ClientID, m.Seq, m.Key, m.Value = d.u(), d.u(), d.str(), d.str()
	case *PutResponse:
		m.Err, m.Leader = d.str(), d.u()
	case *CASRequest:
		m.ClientID, m.Seq, m.Key, m.Expected, m.Value = d.u(), d.u(), d.str(), d.str(), d.str()
	case *CASResponse:
		m.OK, m.Err, m.Leader = d.bool(), d.str(), d.u()
	case *StatusRequest, *ClockResponse:
	case *StatusResponse:
		m.ID, m.Term, m.Leader, m.Commit, m.Applied, m.LastIndex, m.SnapIndex = d.u(), d.u(), d.u(), d.u(), d.u(), d.u(), d.u()
		m.Role = d.str()
		m.ClockOffsetNanos = int64(d.u())
		fmt.Sscan(d.str(), &m.ClockRate)
		n := d.u()
		for i := uint64(0); i < n && d.err == nil; i++ {
			m.Bugs = append(m.Bugs, d.str())
		}
	case *ClockRequest:
		m.JumpNanos = int64(d.u())
		fmt.Sscan(d.str(), &m.Rate)
		m.Reset = d.bool()
	default:
		return fmt.Errorf("rkv codec: cannot unmarshal %T", v)
	}
	return d.err
}

type enc struct{ b []byte }

func (e *enc) u(vs ...uint64) {
	for _, v := range vs {
		e.b = binary.AppendUvarint(e.b, v)
	}
}
func (e *enc) bytes(p []byte) { e.u(uint64(len(p))); e.b = append(e.b, p...) }
func (e *enc) str(s string)   { e.u(uint64(len(s))); e.b = append(e.b, s...) }
func (e *enc) bool(v bool) {
	if v {
		e.b = append(e.b, 1)
	} else {
		e.b = append(e.b, 0)
	}
}

type dec struct {
	b   []byte
	err error
}

var errShort = errors.New("rkv codec: short message")

func (d *dec) u() uint64 {
	if d.err != nil {
		return 0
	}
	v, n := binary.Uvarint(d.b)
	if n <= 0 {
		d.err = errShort
		return 0
	}
	d.b = d.b[n:]
	return v
}

func (d *dec) raw() []byte {
	n := d.u()
	if d.err != nil {
		return nil
	}
	if n > uint64(len(d.b)) {
		d.err = errShort
		return nil
	}
	p := d.b[:n]
	d.b = d.b[n:]
	return p
}

func (d *dec) bytes() []byte {
	p := d.raw()
	if len(p) == 0 {
		return nil
	}
	return append([]byte(nil), p...)
}
func (d *dec) str() string { return string(d.raw()) }
func (d *dec) bool() bool {
	if d.err != nil || len(d.b) == 0 {
		d.err = errShort
		return false
	}
	v := d.b[0] == 1
	d.b = d.b[1:]
	return v
}
