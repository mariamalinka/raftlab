package bench

import (
	"context"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"raftkv/client"
)

// RaftKV pools single-session clients so concurrent requests each have
// their own session (a session's writes must be sequential).
type RaftKV struct {
	conns *client.Conns
	mu    sync.Mutex
	free  []*client.Client
}

func NewRaftKV(addrs map[uint64]string) (*RaftKV, error) {
	conns, err := client.Dial(addrs)
	if err != nil {
		return nil, err
	}
	return &RaftKV{conns: conns}, nil
}

func (b *RaftKV) Name() string { return "raftkv" }

func (b *RaftKV) get() *client.Client {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n := len(b.free); n > 0 {
		c := b.free[n-1]
		b.free = b.free[:n-1]
		return c
	}
	return client.New(b.conns)
}

func (b *RaftKV) put(c *client.Client) {
	b.mu.Lock()
	b.free = append(b.free, c)
	b.mu.Unlock()
}

func (b *RaftKV) Get(ctx context.Context, key string) error {
	c := b.get()
	defer b.put(c)
	_, _, err := c.Get(ctx, key)
	return err
}

func (b *RaftKV) Put(ctx context.Context, key, value string) error {
	c := b.get()
	defer b.put(c)
	return c.Put(ctx, key, value)
}

func (b *RaftKV) Close() { b.conns.Close() }

// Etcd uses the official client; reads are linearizable unless Serializable.
type Etcd struct {
	clis         []*clientv3.Client
	next         uint64
	mu           sync.Mutex
	Serializable bool
}

func NewEtcd(endpoints []string, conns int) (*Etcd, error) {
	b := &Etcd{}
	for range conns {
		cli, err := clientv3.New(clientv3.Config{Logger: zap.NewNop(), Endpoints: endpoints})
		if err != nil {
			return nil, err
		}
		b.clis = append(b.clis, cli)
	}
	return b, nil
}

func (b *Etcd) Name() string { return "etcd" }

func (b *Etcd) cli() *clientv3.Client {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	return b.clis[b.next%uint64(len(b.clis))]
}

func (b *Etcd) Get(ctx context.Context, key string) error {
	var opts []clientv3.OpOption
	if b.Serializable {
		opts = append(opts, clientv3.WithSerializable())
	}
	_, err := b.cli().Get(ctx, key, opts...)
	return err
}

func (b *Etcd) Put(ctx context.Context, key, value string) error {
	_, err := b.cli().Put(ctx, key, value)
	return err
}

func (b *Etcd) Close() {
	for _, c := range b.clis {
		c.Close()
	}
}
