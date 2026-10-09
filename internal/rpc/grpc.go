package rpc

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"raftkv/internal/raft"
)

// KVService is what a node exposes to clients and to the fault harness.
type KVService interface {
	Get(ctx context.Context, req *GetRequest) (*GetResponse, error)
	Put(ctx context.Context, req *PutRequest) (*PutResponse, error)
	CAS(ctx context.Context, req *CASRequest) (*CASResponse, error)
	Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error)
	Clock(ctx context.Context, req *ClockRequest) (*ClockResponse, error)
}

// RaftService is the peer-to-peer side; *raft.Raft implements it.
type RaftService interface {
	HandleRequestVote(*raft.RequestVoteRequest) *raft.RequestVoteResponse
	HandleAppendEntries(*raft.AppendEntriesRequest) *raft.AppendEntriesResponse
	HandleInstallSnapshot(*raft.InstallSnapshotRequest) *raft.InstallSnapshotResponse
}

func method[Req, Resp any](name string, f func(srv any, ctx context.Context, req *Req) (*Resp, error)) grpc.MethodDesc {
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			req := new(Req)
			if err := dec(req); err != nil {
				return nil, err
			}
			return f(srv, ctx, req)
		},
	}
}

var raftDesc = grpc.ServiceDesc{
	ServiceName: "raftkv.Raft",
	HandlerType: (*RaftService)(nil),
	Methods: []grpc.MethodDesc{
		method("RequestVote", func(s any, _ context.Context, r *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
			return s.(RaftService).HandleRequestVote(r), nil
		}),
		method("AppendEntries", func(s any, _ context.Context, r *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
			return s.(RaftService).HandleAppendEntries(r), nil
		}),
		method("InstallSnapshot", func(s any, _ context.Context, r *raft.InstallSnapshotRequest) (*raft.InstallSnapshotResponse, error) {
			return s.(RaftService).HandleInstallSnapshot(r), nil
		}),
	},
}

var kvDesc = grpc.ServiceDesc{
	ServiceName: "raftkv.KV",
	HandlerType: (*KVService)(nil),
	Methods: []grpc.MethodDesc{
		method("Get", func(s any, ctx context.Context, r *GetRequest) (*GetResponse, error) {
			return s.(KVService).Get(ctx, r)
		}),
		method("Put", func(s any, ctx context.Context, r *PutRequest) (*PutResponse, error) {
			return s.(KVService).Put(ctx, r)
		}),
		method("CAS", func(s any, ctx context.Context, r *CASRequest) (*CASResponse, error) {
			return s.(KVService).CAS(ctx, r)
		}),
		method("Status", func(s any, ctx context.Context, r *StatusRequest) (*StatusResponse, error) {
			return s.(KVService).Status(ctx, r)
		}),
		method("Clock", func(s any, ctx context.Context, r *ClockRequest) (*ClockResponse, error) {
			return s.(KVService).Clock(ctx, r)
		}),
	},
}

// NewServer returns a gRPC server exposing both services.
func NewServer(raftSvc RaftService, kvSvc KVService) *grpc.Server {
	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 200 * time.Millisecond, PermitWithoutStream: true}),
		grpc.MaxRecvMsgSize(64<<20),
	)
	s.RegisterService(&raftDesc, raftSvc)
	s.RegisterService(&kvDesc, kvSvc)
	return s
}

// Dial connects to addr. If localIP is set, outgoing connections are bound to
// it so the harness can partition nodes with iptables rules on source and
// destination address.
func Dial(addr, localIP string) (*grpc.ClientConn, error) {
	dialer := &net.Dialer{Timeout: time.Second}
	if localIP != "" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(localIP)}
	}
	return grpc.NewClient("passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithNoProxy(),
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", a)
		}),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.Config{BaseDelay: 50 * time.Millisecond, Multiplier: 1.6, Jitter: 0.2, MaxDelay: 500 * time.Millisecond},
			MinConnectTimeout: time.Second,
		}),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: time.Second, Timeout: time.Second, PermitWithoutStream: true}),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(codecName), grpc.MaxCallRecvMsgSize(64<<20)),
	)
}

// Transport implements raft.Transport over gRPC.
type Transport struct {
	localIP string
	addrs   map[uint64]string
	mu      sync.Mutex
	conns   map[uint64]*grpc.ClientConn
}

func NewTransport(localIP string, addrs map[uint64]string) *Transport {
	return &Transport{localIP: localIP, addrs: addrs, conns: map[uint64]*grpc.ClientConn{}}
}

func (t *Transport) conn(to uint64) (*grpc.ClientConn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.conns[to]; ok {
		return c, nil
	}
	c, err := Dial(t.addrs[to], t.localIP)
	if err != nil {
		return nil, err
	}
	t.conns[to] = c
	return c, nil
}

func invoke[Resp any](t *Transport, ctx context.Context, to uint64, name string, req any) (*Resp, error) {
	c, err := t.conn(to)
	if err != nil {
		return nil, err
	}
	resp := new(Resp)
	if err := c.Invoke(ctx, "/raftkv.Raft/"+name, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (t *Transport) RequestVote(ctx context.Context, to uint64, req *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	return invoke[raft.RequestVoteResponse](t, ctx, to, "RequestVote", req)
}

func (t *Transport) AppendEntries(ctx context.Context, to uint64, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	return invoke[raft.AppendEntriesResponse](t, ctx, to, "AppendEntries", req)
}

func (t *Transport) InstallSnapshot(ctx context.Context, to uint64, req *raft.InstallSnapshotRequest) (*raft.InstallSnapshotResponse, error) {
	return invoke[raft.InstallSnapshotResponse](t, ctx, to, "InstallSnapshot", req)
}

func (t *Transport) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range t.conns {
		c.Close()
	}
}

// KVClient calls one node's client API.
type KVClient struct{ cc *grpc.ClientConn }

func NewKVClient(addr string) (*KVClient, error) {
	cc, err := Dial(addr, "")
	if err != nil {
		return nil, err
	}
	return &KVClient{cc: cc}, nil
}

func call[Resp any](c *KVClient, ctx context.Context, name string, req any) (*Resp, error) {
	resp := new(Resp)
	if err := c.cc.Invoke(ctx, "/raftkv.KV/"+name, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *KVClient) Get(ctx context.Context, r *GetRequest) (*GetResponse, error) {
	return call[GetResponse](c, ctx, "Get", r)
}
func (c *KVClient) Put(ctx context.Context, r *PutRequest) (*PutResponse, error) {
	return call[PutResponse](c, ctx, "Put", r)
}
func (c *KVClient) CAS(ctx context.Context, r *CASRequest) (*CASResponse, error) {
	return call[CASResponse](c, ctx, "CAS", r)
}
func (c *KVClient) Status(ctx context.Context) (*StatusResponse, error) {
	return call[StatusResponse](c, ctx, "Status", &StatusRequest{})
}
func (c *KVClient) Clock(ctx context.Context, r *ClockRequest) (*ClockResponse, error) {
	return call[ClockResponse](c, ctx, "Clock", r)
}
func (c *KVClient) Close() error { return c.cc.Close() }
