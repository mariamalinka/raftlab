// Package client is the raftkv client library: it finds the leader, retries,
// and tags writes with a client ID and sequence number so the servers can
// drop duplicates.
package client

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"time"

	"raftkv/internal/rpc"
)

var (
	// ErrIndeterminate means the operation may or may not have taken effect.
	// The harness records it as such, never as a failure.
	ErrIndeterminate = errors.New("client: indeterminate")
	// ErrUnavailable means the operation definitely did not take effect.
	ErrUnavailable = errors.New("client: unavailable")
)

// Conns holds one connection per node; share it between many Clients.
type Conns struct {
	ids   []uint64
	nodes map[uint64]*rpc.KVClient
}

func Dial(addrs map[uint64]string) (*Conns, error) {
	c := &Conns{nodes: map[uint64]*rpc.KVClient{}}
	for id, a := range addrs {
		kc, err := rpc.NewKVClient(a)
		if err != nil {
			return nil, err
		}
		c.ids = append(c.ids, id)
		c.nodes[id] = kc
	}
	sort.Slice(c.ids, func(i, j int) bool { return c.ids[i] < c.ids[j] })
	return c, nil
}

func (c *Conns) Close() {
	for _, n := range c.nodes {
		n.Close()
	}
}

func (c *Conns) Node(id uint64) *rpc.KVClient { return c.nodes[id] }
func (c *Conns) IDs() []uint64                { return c.ids }

// Client is a single logical client (one session). It is not safe for
// concurrent use: each op runs to completion before the next.
type Client struct {
	conns   *Conns
	id      uint64
	seq     uint64
	leader  uint64
	// Home, if set, is the node every operation tries first, the way a
	// Jepsen client process is bound to one node. Followers redirect.
	Home uint64
	// AttemptTimeout bounds one RPC; the caller's context bounds the op.
	AttemptTimeout time.Duration
}

func New(conns *Conns) *Client {
	return &Client{conns: conns, id: rand.Uint64() | 1, AttemptTimeout: 500 * time.Millisecond}
}

func (c *Client) ID() uint64 { return c.id }

// SetLeader pins the first node tried (useful for single-node benchmarks).
func (c *Client) SetLeader(id uint64) { c.leader = id }

// do runs attempt against nodes until it succeeds or ctx ends. attempt returns
// the server's error string and leader hint; maybeApplied is set if the
// request might have reached a leader that proposed it.
func (c *Client) do(ctx context.Context, attempt func(ctx context.Context, n *rpc.KVClient) (string, uint64, error)) (maybeApplied bool, err error) {
	tried := 0
	if c.Home != 0 {
		c.leader = c.Home
	}
	for {
		if ctx.Err() != nil {
			if maybeApplied {
				return true, ErrIndeterminate
			}
			return false, ErrUnavailable
		}
		target := c.leader
		if target == 0 {
			ids := c.conns.ids
			target = ids[tried%len(ids)]
		}
		tried++
		actx, cancel := context.WithTimeout(ctx, c.AttemptTimeout)
		serr, hint, rerr := attempt(actx, c.conns.nodes[target])
		cancel()
		switch {
		case rerr != nil:
			maybeApplied = true // the request may have been delivered
			c.leader = 0
		case serr == "":
			return maybeApplied, nil
		case serr == rpc.ErrNotLeader:
			if hint != 0 && hint != target {
				c.leader = hint
			} else {
				c.leader = 0
			}
			// A leader can propose, lose leadership and answer not_leader;
			// the entry may still commit later.
			maybeApplied = true
		default: // timeout or stopped
			maybeApplied = true
			c.leader = 0
		}
		if c.leader == 0 {
			select {
			case <-time.After(10 * time.Millisecond):
			case <-ctx.Done():
			}
		}
	}
}

// Get reads key. Reads have no side effects, so a failed read is just a failure.
func (c *Client) Get(ctx context.Context, key string) (value string, found bool, err error) {
	_, err = c.do(ctx, func(ctx context.Context, n *rpc.KVClient) (string, uint64, error) {
		r, err := n.Get(ctx, &rpc.GetRequest{Key: key})
		if err != nil {
			return "", 0, err
		}
		value, found = r.Value, r.Found
		return r.Err, r.Leader, nil
	})
	if err != nil {
		return "", false, ErrUnavailable
	}
	return value, found, nil
}

func (c *Client) Put(ctx context.Context, key, value string) error {
	c.seq++
	req := &rpc.PutRequest{ClientID: c.id, Seq: c.seq, Key: key, Value: value}
	_, err := c.do(ctx, func(ctx context.Context, n *rpc.KVClient) (string, uint64, error) {
		r, err := n.Put(ctx, req)
		if err != nil {
			return "", 0, err
		}
		return r.Err, r.Leader, nil
	})
	return err
}

// CAS sets key to value if it currently holds expected ("" = missing).
func (c *Client) CAS(ctx context.Context, key, expected, value string) (bool, error) {
	c.seq++
	req := &rpc.CASRequest{ClientID: c.id, Seq: c.seq, Key: key, Expected: expected, Value: value}
	var ok bool
	_, err := c.do(ctx, func(ctx context.Context, n *rpc.KVClient) (string, uint64, error) {
		r, err := n.CAS(ctx, req)
		if err != nil {
			return "", 0, err
		}
		ok = r.OK
		return r.Err, r.Leader, nil
	})
	return ok, err
}
