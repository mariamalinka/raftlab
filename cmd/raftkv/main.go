// Command raftkv runs one node of the Raft key-value store.
//
//	raftkv -id 1 -peers 1=127.0.0.11:7000,2=127.0.0.12:7000,3=127.0.0.13:7000 -data /tmp/n1
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"raftkv/internal/bugs"
	"raftkv/internal/kv"
	"raftkv/internal/server"
	"raftkv/internal/wal"
)

func parsePeers(s string) (map[uint64]string, error) {
	peers := map[uint64]string{}
	for _, p := range strings.Split(s, ",") {
		id, addr, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok {
			return nil, fmt.Errorf("bad peer %q, want id=host:port", p)
		}
		n, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return nil, err
		}
		peers[n] = addr
	}
	return peers, nil
}

func main() {
	var (
		id        = flag.Uint64("id", 1, "node id (1-based)")
		peersFlag = flag.String("peers", "", "comma-separated id=host:port for every node")
		dataDir   = flag.String("data", "", "data directory")
		readMode  = flag.String("read-mode", "readindex", "read path: log, readindex or lease")
		election  = flag.Duration("election-timeout", 1000*time.Millisecond, "minimum election timeout (etcd default 1s)")
		heartbeat = flag.Duration("heartbeat", 100*time.Millisecond, "heartbeat interval (etcd default 100ms)")
		drift     = flag.Float64("lease-drift", 0.1, "clock drift bound for leases, as a fraction of the election timeout")
		batch     = flag.Int("batch", 256, "max entries per AppendEntries / fsync group; 1 disables batching")
		pipeline  = flag.Bool("pipeline", true, "pipeline AppendEntries")
		inflight  = flag.Int("inflight", 8, "max in-flight AppendEntries per follower when pipelining")
		fsync     = flag.Bool("fsync", true, "fsync the WAL before acknowledging")
		lazyfs    = flag.Bool("lazyfs", false, "lose unsynced WAL writes on crash (LazyFS stand-in)")
		snapBytes = flag.Int64("snapshot-bytes", 64<<20, "snapshot once the WAL exceeds this size; 0 disables")
		logLevel  = flag.String("log-level", "info", "debug, info, warn or error")
	)
	flag.Parse()
	peers, err := parsePeers(*peersFlag)
	if err != nil || *dataDir == "" {
		fmt.Fprintln(os.Stderr, "need -peers and -data:", err)
		os.Exit(2)
	}
	mode, err := kv.ParseReadMode(*readMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	if bugs.Compiled {
		logger.Warn("bug-injection build", "bugs", bugs.Active())
	}

	srv, err := server.Start(server.Options{
		ID: *id, Peers: peers, DataDir: *dataDir, ReadMode: mode,
		ElectionTimeout: *election, HeartbeatInterval: *heartbeat, LeaseDriftBound: *drift,
		MaxBatch: *batch, Pipeline: *pipeline, MaxInflight: *inflight, SnapshotBytes: *snapBytes,
		WAL:    wal.Options{NoSync: !*fsync, Lazy: *lazyfs},
		Logger: logger,
	})
	if err != nil {
		logger.Error("start failed", "err", err)
		os.Exit(1)
	}
	logger.Info("serving", "node", *id, "addr", peers[*id], "read_mode", mode)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	srv.Stop()
}
