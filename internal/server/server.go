// Package server wires a KV store, its Raft node and the gRPC endpoints into
// one process.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"

	"raftkv/internal/bugs"
	"raftkv/internal/clock"
	"raftkv/internal/kv"
	"raftkv/internal/raft"
	"raftkv/internal/rpc"
	"raftkv/internal/wal"
)

type Options struct {
	ID       uint64
	Peers    map[uint64]string // id -> host:port, including this node
	DataDir  string
	ReadMode kv.ReadMode

	ElectionTimeout   time.Duration
	HeartbeatInterval time.Duration
	LeaseDriftBound   float64
	MaxBatch          int
	Pipeline          bool
	MaxInflight       int
	SnapshotBytes     int64
	// RequestTimeout bounds how long the server works on one client request.
	RequestTimeout time.Duration

	WAL    wal.Options
	Logger *slog.Logger
}

type Server struct {
	opts  Options
	store *kv.Store
	clock *clock.Skewable
	grpc  *grpc.Server
	trans *rpc.Transport
	w     *wal.WAL
	lis   net.Listener
}

func Start(opts Options) (*Server, error) {
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 2 * time.Second
	}
	addr := opts.Peers[opts.ID]
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	w, err := wal.Open(opts.DataDir, opts.WAL)
	if err != nil {
		return nil, err
	}
	var ids []uint64
	for id := range opts.Peers {
		ids = append(ids, id)
	}
	clk := clock.NewSkewable()
	trans := rpc.NewTransport(host, opts.Peers)
	store, err := kv.New(kv.Config{
		Raft: raft.Config{
			ID:                opts.ID,
			Peers:             ids,
			ElectionTimeout:   opts.ElectionTimeout,
			HeartbeatInterval: opts.HeartbeatInterval,
			LeaseDriftBound:   opts.LeaseDriftBound,
			MaxBatch:          opts.MaxBatch,
			Pipeline:          opts.Pipeline,
			MaxInflight:       opts.MaxInflight,
			Clock:             clk,
			Storage:           w,
			Transport:         trans,
		},
		ReadMode:      opts.ReadMode,
		SnapshotBytes: opts.SnapshotBytes,
		Logger:        opts.Logger,
	})
	if err != nil {
		return nil, err
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		store.Stop()
		return nil, err
	}
	s := &Server{opts: opts, store: store, clock: clk, trans: trans, w: w, lis: lis}
	s.grpc = rpc.NewServer(store.Raft(), s)
	go s.grpc.Serve(lis)
	return s, nil
}

func (s *Server) Stop() {
	s.grpc.Stop()
	s.store.Stop()
	s.trans.Close()
	s.w.Close()
}

func errString(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, kv.ErrNotLeader):
		return rpc.ErrNotLeader
	case errors.Is(err, kv.ErrTimeout):
		return rpc.ErrTimeout
	case errors.Is(err, kv.ErrStopped):
		return rpc.ErrStopped
	}
	return err.Error()
}

func (s *Server) ctx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.opts.RequestTimeout)
}

func (s *Server) leaderHint() uint64 { return s.store.Raft().Status().Leader }

func (s *Server) Get(ctx context.Context, req *rpc.GetRequest) (*rpc.GetResponse, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	r, err := s.store.Get(ctx, req.Key)
	return &rpc.GetResponse{Value: r.Value, Found: r.Found, Err: errString(err), Leader: s.leaderHint()}, nil
}

func (s *Server) Put(ctx context.Context, req *rpc.PutRequest) (*rpc.PutResponse, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	err := s.store.Put(ctx, req.ClientID, req.Seq, req.Key, req.Value)
	return &rpc.PutResponse{Err: errString(err), Leader: s.leaderHint()}, nil
}

func (s *Server) CAS(ctx context.Context, req *rpc.CASRequest) (*rpc.CASResponse, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	ok, err := s.store.CAS(ctx, req.ClientID, req.Seq, req.Key, req.Expected, req.Value)
	return &rpc.CASResponse{OK: ok, Err: errString(err), Leader: s.leaderHint()}, nil
}

func (s *Server) Status(ctx context.Context, _ *rpc.StatusRequest) (*rpc.StatusResponse, error) {
	st := s.store.Raft().Status()
	off, rate := s.clock.Skew()
	return &rpc.StatusResponse{
		ID: st.ID, Term: st.Term, Leader: st.Leader, Commit: st.Commit, Applied: st.Applied,
		LastIndex: st.LastIndex, SnapIndex: st.SnapIndex, Role: st.Role.String(),
		ClockOffsetNanos: int64(off), ClockRate: rate, Bugs: bugs.Active(),
	}, nil
}

func (s *Server) Clock(ctx context.Context, req *rpc.ClockRequest) (*rpc.ClockResponse, error) {
	switch {
	case req.Reset:
		s.clock.Reset()
	default:
		if req.JumpNanos != 0 {
			s.clock.Jump(time.Duration(req.JumpNanos))
		}
		if req.Rate > 0 {
			s.clock.SetRate(req.Rate)
		}
	}
	off, rate := s.clock.Skew()
	s.opts.Logger.Info("clock skewed", "node", s.opts.ID, "offset", off, "rate", rate)
	return &rpc.ClockResponse{}, nil
}
