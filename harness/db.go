package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"raftkv/client"
	"raftkv/internal/rpc"
)

var (
	ErrUnsupported   = errors.New("fault not supported by this database")
	ErrIndeterminate = errors.New("indeterminate")
)

// Client is one logical client process. Reads return an error on failure;
// writes return nil, ErrIndeterminate, or another error meaning "definitely
// did not happen".
type Client interface {
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Put(ctx context.Context, key, value string) error
	CAS(ctx context.Context, key, expected, value string) (bool, error)
	Close()
}

// DB is a cluster under test.
type DB interface {
	Name() string
	Setup() error
	Start(i int) error
	Kill(i int) error
	Pause(i int) error
	Resume(i int) error
	SkewClock(i int, jump time.Duration, rate float64) error
	ResetClocks() error
	Leader(ctx context.Context) (int, error)
	NewClient() (Client, error)
	Teardown()
}

// procs manages one OS process per node, each in its node's namespace.
type procs struct {
	n    int
	dir  string
	mu   sync.Mutex
	cmds []*exec.Cmd
	done []chan struct{}
	argv func(i int) []string
	env  []string
}

func newProcs(n int, dir string, argv func(i int) []string, env []string) *procs {
	return &procs{n: n, dir: dir, cmds: make([]*exec.Cmd, n), done: make([]chan struct{}, n), argv: argv, env: env}
}

func (p *procs) start(i int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmds[i] != nil {
		return nil
	}
	logf, err := os.OpenFile(filepath.Join(p.dir, fmt.Sprintf("n%d.log", i+1)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	args := append([]string{"netns", "exec", nsName(i)}, p.argv(i)...)
	cmd := exec.Command("ip", args...)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.Env = append(os.Environ(), p.env...)
	if err := cmd.Start(); err != nil {
		logf.Close()
		return err
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); logf.Close(); close(done) }()
	p.cmds[i], p.done[i] = cmd, done
	return nil
}

func (p *procs) signal(i int, sig syscall.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmds[i] == nil {
		return nil
	}
	return p.cmds[i].Process.Signal(sig)
}

func (p *procs) kill(i int) error {
	p.mu.Lock()
	cmd, done := p.cmds[i], p.done[i]
	p.cmds[i] = nil
	p.mu.Unlock()
	if cmd == nil {
		return nil
	}
	cmd.Process.Signal(syscall.SIGKILL)
	<-done
	return nil
}

func (p *procs) killAll() {
	for i := range p.n {
		p.kill(i)
	}
}

func (p *procs) running(i int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmds[i] != nil
}

// ---------------- raftkv ----------------

type RaftKVOptions struct {
	Bin             string // node binary (built with -tags bugs if Bugs is set)
	ReadMode        string
	Bugs            []string
	LazyFS          bool
	ElectionTimeout time.Duration
	Heartbeat       time.Duration
	ExtraArgs       []string
}

type RaftKV struct {
	opts  RaftKVOptions
	n     int
	dir   string
	procs *procs
	conns *client.Conns
	next  int
}

const raftkvPort = 7000

func NewRaftKV(n int, dir string, opts RaftKVOptions) *RaftKV {
	db := &RaftKV{opts: opts, n: n, dir: dir}
	var peers []string
	for i := range n {
		peers = append(peers, fmt.Sprintf("%d=%s:%d", i+1, nodeIP(i), raftkvPort))
	}
	argv := func(i int) []string {
		a := []string{opts.Bin,
			"-id", strconv.Itoa(i + 1),
			"-peers", strings.Join(peers, ","),
			"-data", filepath.Join(dir, fmt.Sprintf("data%d", i+1)),
			"-read-mode", opts.ReadMode,
			"-election-timeout", opts.ElectionTimeout.String(),
			"-heartbeat", opts.Heartbeat.String(),
			// Small snapshots so tests exercise compaction and InstallSnapshot.
			"-snapshot-bytes", "262144",
		}
		if opts.LazyFS {
			a = append(a, "-lazyfs")
		}
		return append(a, opts.ExtraArgs...)
	}
	var env []string
	if len(opts.Bugs) > 0 {
		env = append(env, "RAFTKV_BUGS="+strings.Join(opts.Bugs, ","))
	}
	db.procs = newProcs(n, dir, argv, env)
	return db
}

func (d *RaftKV) Name() string { return "raftkv" }

func (d *RaftKV) Setup() error {
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return err
	}
	for i := range d.n {
		if err := d.procs.start(i); err != nil {
			return err
		}
	}
	addrs := map[uint64]string{}
	for i := range d.n {
		addrs[uint64(i+1)] = fmt.Sprintf("%s:%d", nodeIP(i), raftkvPort)
	}
	conns, err := client.Dial(addrs)
	if err != nil {
		return err
	}
	d.conns = conns
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, err = d.Leader(ctx)
	return err
}

func (d *RaftKV) Start(i int) error  { return d.procs.start(i) }
func (d *RaftKV) Kill(i int) error   { return d.procs.kill(i) }
func (d *RaftKV) Pause(i int) error  { return d.procs.signal(i, syscall.SIGSTOP) }
func (d *RaftKV) Resume(i int) error { return d.procs.signal(i, syscall.SIGCONT) }

func (d *RaftKV) SkewClock(i int, jump time.Duration, rate float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := d.conns.Node(uint64(i+1)).Clock(ctx, &rpc.ClockRequest{JumpNanos: int64(jump), Rate: rate})
	return err
}

func (d *RaftKV) ResetClocks() error {
	for i := range d.n {
		if !d.procs.running(i) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		d.conns.Node(uint64(i+1)).Clock(ctx, &rpc.ClockRequest{Reset: true})
		cancel()
	}
	return nil
}

// Leader polls node status until one node reports itself leader.
func (d *RaftKV) Leader(ctx context.Context) (int, error) {
	for {
		best, bestTerm := -1, uint64(0)
		for i := range d.n {
			if !d.procs.running(i) {
				continue
			}
			c, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			st, err := d.conns.Node(uint64(i + 1)).Status(c)
			cancel()
			if err == nil && st.Role == "leader" && st.Term >= bestTerm {
				best, bestTerm = i, st.Term
			}
		}
		if best >= 0 {
			return best, nil
		}
		select {
		case <-ctx.Done():
			return -1, errors.New("no leader")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (d *RaftKV) NewClient() (Client, error) {
	c := client.New(d.conns)
	c.Home = uint64(d.next%d.n + 1)
	d.next++
	return raftkvClient{c}, nil
}

func (d *RaftKV) Teardown() {
	d.procs.killAll()
	if d.conns != nil {
		d.conns.Close()
	}
}

type raftkvClient struct{ c *client.Client }

func (c raftkvClient) Get(ctx context.Context, key string) (string, bool, error) {
	return c.c.Get(ctx, key)
}

func mapClientErr(err error) error {
	if errors.Is(err, client.ErrIndeterminate) {
		return ErrIndeterminate
	}
	return err
}

func (c raftkvClient) Put(ctx context.Context, key, value string) error {
	return mapClientErr(c.c.Put(ctx, key, value))
}

func (c raftkvClient) CAS(ctx context.Context, key, expected, value string) (bool, error) {
	ok, err := c.c.CAS(ctx, key, expected, value)
	return ok, mapClientErr(err)
}

func (c raftkvClient) Close() {}

// ---------------- etcd ----------------

type EtcdOptions struct {
	Bin string
	// Serializable reads skip the quorum check (unsafe; experiment E8).
	Serializable    bool
	ElectionTimeout time.Duration
	Heartbeat       time.Duration
	NoFsync         bool
}

type Etcd struct {
	opts      EtcdOptions
	n         int
	dir       string
	procs     *procs
	endpoints []string
	next      int
}

func NewEtcd(n int, dir string, opts EtcdOptions) *Etcd {
	d := &Etcd{opts: opts, n: n, dir: dir}
	var cluster []string
	for i := range n {
		cluster = append(cluster, fmt.Sprintf("n%d=http://%s:2380", i+1, nodeIP(i)))
		d.endpoints = append(d.endpoints, fmt.Sprintf("http://%s:2379", nodeIP(i)))
	}
	argv := func(i int) []string {
		ip := nodeIP(i)
		a := []string{opts.Bin,
			"--name", fmt.Sprintf("n%d", i+1),
			"--data-dir", filepath.Join(dir, fmt.Sprintf("data%d", i+1)),
			"--listen-peer-urls", "http://" + ip + ":2380",
			"--initial-advertise-peer-urls", "http://" + ip + ":2380",
			"--listen-client-urls", "http://" + ip + ":2379",
			"--advertise-client-urls", "http://" + ip + ":2379",
			"--initial-cluster", strings.Join(cluster, ","),
			"--initial-cluster-state", "new",
			"--initial-cluster-token", "raftlab",
			"--heartbeat-interval", strconv.Itoa(int(opts.Heartbeat / time.Millisecond)),
			"--election-timeout", strconv.Itoa(int(opts.ElectionTimeout / time.Millisecond)),
			"--snapshot-count", "10000",
			"--log-level", "warn",
		}
		if opts.NoFsync {
			a = append(a, "--unsafe-no-fsync")
		}
		return a
	}
	d.procs = newProcs(n, dir, argv, nil)
	return d
}

func (d *Etcd) Name() string { return "etcd" }

func (d *Etcd) Setup() error {
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return err
	}
	for i := range d.n {
		if err := d.procs.start(i); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := d.Leader(ctx)
	return err
}

func (d *Etcd) Start(i int) error  { return d.procs.start(i) }
func (d *Etcd) Kill(i int) error   { return d.procs.kill(i) }
func (d *Etcd) Pause(i int) error  { return d.procs.signal(i, syscall.SIGSTOP) }
func (d *Etcd) Resume(i int) error { return d.procs.signal(i, syscall.SIGCONT) }

func (d *Etcd) SkewClock(int, time.Duration, float64) error { return ErrUnsupported }
func (d *Etcd) ResetClocks() error                          { return nil }

func (d *Etcd) Leader(ctx context.Context) (int, error) {
	cli, err := clientv3.New(clientv3.Config{Logger: zap.NewNop(), Endpoints: d.endpoints, DialTimeout: time.Second})
	if err != nil {
		return -1, err
	}
	defer cli.Close()
	for {
		for i, ep := range d.endpoints {
			if !d.procs.running(i) {
				continue
			}
			c, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
			st, err := cli.Status(c, ep)
			cancel()
			if err == nil && st.Leader != 0 && st.Leader == st.Header.MemberId {
				return i, nil
			}
		}
		select {
		case <-ctx.Done():
			return -1, errors.New("no leader")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (d *Etcd) NewClient() (Client, error) {
	// Each client process talks to one node, as in Jepsen's etcd test.
	ep := d.endpoints[d.next%d.n]
	d.next++
	cli, err := clientv3.New(clientv3.Config{Logger: zap.NewNop(),
		Endpoints:   []string{ep},
		DialTimeout: time.Second,
		// Fail fast: the harness decides what a timeout means.
		MaxCallSendMsgSize: 4 << 20,
	})
	if err != nil {
		return nil, err
	}
	return &etcdClient{cli: cli, serializable: d.opts.Serializable}, nil
}

func (d *Etcd) Teardown() { d.procs.killAll() }

type etcdClient struct {
	cli          *clientv3.Client
	serializable bool
}

func (c *etcdClient) Get(ctx context.Context, key string) (string, bool, error) {
	var opts []clientv3.OpOption
	if c.serializable {
		opts = append(opts, clientv3.WithSerializable())
	}
	r, err := c.cli.Get(ctx, key, opts...)
	if err != nil {
		return "", false, err
	}
	if len(r.Kvs) == 0 {
		return "", false, nil
	}
	return string(r.Kvs[0].Value), true, nil
}

func (c *etcdClient) Put(ctx context.Context, key, value string) error {
	if _, err := c.cli.Put(ctx, key, value); err != nil {
		return ErrIndeterminate
	}
	return nil
}

func (c *etcdClient) CAS(ctx context.Context, key, expected, value string) (bool, error) {
	cmp := clientv3.Compare(clientv3.Value(key), "=", expected)
	if expected == "" {
		cmp = clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	}
	r, err := c.cli.Txn(ctx).If(cmp).Then(clientv3.OpPut(key, value)).Commit()
	if err != nil {
		return false, ErrIndeterminate
	}
	return r.Succeeded, nil
}

func (c *etcdClient) Close() { c.cli.Close() }

// Addrs returns each node's client address.
func (d *RaftKV) Addrs() map[uint64]string {
	addrs := map[uint64]string{}
	for i := range d.n {
		addrs[uint64(i+1)] = fmt.Sprintf("%s:%d", nodeIP(i), raftkvPort)
	}
	return addrs
}

// Applied reports node i's applied index.
func (d *RaftKV) Applied(ctx context.Context, i int) (uint64, error) {
	st, err := d.conns.Node(uint64(i + 1)).Status(ctx)
	if err != nil {
		return 0, err
	}
	return st.Applied, nil
}

func (d *Etcd) Endpoints() []string { return d.endpoints }

// Applied reports node i's applied raft index.
func (d *Etcd) Applied(ctx context.Context, i int) (uint64, error) {
	cli, err := clientv3.New(clientv3.Config{Logger: zap.NewNop(), Endpoints: []string{d.endpoints[i]}, DialTimeout: time.Second})
	if err != nil {
		return 0, err
	}
	defer cli.Close()
	st, err := cli.Status(ctx, d.endpoints[i])
	if err != nil {
		return 0, err
	}
	return st.RaftAppliedIndex, nil
}

// ClusterDB is a DB that can also report replication progress.
type ClusterDB interface {
	DB
	Applied(ctx context.Context, i int) (uint64, error)
}
