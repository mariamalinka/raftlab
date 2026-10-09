// Command loadgen runs the performance experiments (E3, E4, E5 speed side,
// E7) against raftkv or etcd with identical clients and workloads. It
// starts the cluster itself (one network namespace per node; needs root).
//
//	loadgen -db raftkv -rates 500,1000,2000,4000 -read-frac 0.95 -out results/E3/raftkv-95
//	loadgen -db etcd -mode failover -trials 10 -out results/E4/etcd
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"raftkv/bench"
	"raftkv/harness"
)

func main() {
	var (
		db        = flag.String("db", "raftkv", "raftkv or etcd")
		mode      = flag.String("mode", "sweep", "sweep (latency vs load) or failover (E4)")
		nodes     = flag.Int("nodes", 5, "cluster size")
		bin       = flag.String("bin", "/tmp/raftkv-bin", "directory with raftkv and etcd binaries")
		rates     = flag.String("rates", "500,1000,2000,4000,8000", "offered loads (ops/s) for sweep mode")
		reps      = flag.Int("reps", 5, "repetitions per load point")
		dur       = flag.Duration("duration", 20*time.Second, "measured time per point")
		warmup    = flag.Duration("warmup", 5*time.Second, "unmeasured warmup per point")
		readFrac  = flag.Float64("read-frac", 0.95, "fraction of reads")
		valueSize = flag.Int("value-size", 128, "value size in bytes")
		keys      = flag.Int("keys", 10000, "key space")
		zipf      = flag.Bool("zipf", false, "Zipfian key choice instead of uniform")
		election  = flag.Duration("election-timeout", time.Second, "election timeout (both systems)")
		heartbeat = flag.Duration("heartbeat", 100*time.Millisecond, "heartbeat interval (both systems)")
		readMode  = flag.String("read-mode", "readindex", "raftkv read path")
		raftArgs  = flag.String("raftkv-args", "", "extra raftkv flags, e.g. \"-batch=1 -pipeline=false -fsync=false\"")
		noFsync   = flag.Bool("etcd-no-fsync", false, "etcd --unsafe-no-fsync (labelled extra runs only)")
		serial    = flag.Bool("etcd-serializable", false, "etcd serializable reads")
		trials    = flag.Int("trials", 10, "leader kills in failover mode")
		failRate  = flag.Float64("failover-rate", 500, "offered write load during failover trials")
		out       = flag.String("out", "results/bench-scratch", "output directory")
	)
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		die(err)
	}
	dataDir := filepath.Join(os.TempDir(), "raftlab-bench")
	os.RemoveAll(dataDir)
	defer os.RemoveAll(dataDir)
	if _, err := harness.SetupNetwork(*nodes); err != nil {
		die(err)
	}
	defer harness.TeardownNetwork(*nodes)

	var cdb harness.ClusterDB
	var backend bench.Backend
	switch *db {
	case "raftkv":
		d := harness.NewRaftKV(*nodes, dataDir, harness.RaftKVOptions{
			Bin: filepath.Join(*bin, "raftkv"), ReadMode: *readMode,
			ElectionTimeout: *election, Heartbeat: *heartbeat, ExtraArgs: strings.Fields(*raftArgs),
		})
		must(d.Setup())
		b, err := bench.NewRaftKV(d.Addrs())
		must(err)
		cdb, backend = d, b
	case "etcd":
		d := harness.NewEtcd(*nodes, dataDir, harness.EtcdOptions{
			Bin: filepath.Join(*bin, "etcd"), ElectionTimeout: *election, Heartbeat: *heartbeat, NoFsync: *noFsync,
		})
		must(d.Setup())
		b, err := bench.NewEtcd(d.Endpoints(), 8)
		must(err)
		b.Serializable = *serial
		cdb, backend = d, b
	default:
		die(fmt.Errorf("unknown db %q", *db))
	}
	defer cdb.Teardown()
	defer backend.Close()

	ctx := context.Background()
	writeJSON(filepath.Join(*out, "config.json"), map[string]any{
		"db": *db, "mode": *mode, "nodes": *nodes, "read_frac": *readFrac, "value_size": *valueSize,
		"keys": *keys, "zipf": *zipf, "election_timeout": election.String(), "heartbeat": heartbeat.String(),
		"read_mode": *readMode, "raftkv_args": *raftArgs, "etcd_no_fsync": *noFsync,
		"etcd_serializable": *serial, "duration": dur.String(), "warmup": warmup.String(),
		"etcd_version": "3.7.2", "started": time.Now().UTC().Format(time.RFC3339),
	})
	must(bench.Preload(ctx, backend, *keys, *valueSize, 64))

	switch *mode {
	case "sweep":
		f, err := os.Create(filepath.Join(*out, "points.jsonl"))
		must(err)
		defer f.Close()
		enc := json.NewEncoder(f)
		for _, rs := range strings.Split(*rates, ",") {
			rate, err := strconv.ParseFloat(strings.TrimSpace(rs), 64)
			must(err)
			for rep := 1; rep <= *reps; rep++ {
				r := bench.Run(ctx, backend, bench.Params{
					Rate: rate, Duration: *dur, Warmup: *warmup, ReadFrac: *readFrac,
					ValueSize: *valueSize, Keys: *keys, Zipf: *zipf,
				})
				row := map[string]any{"rep": rep, "result": r}
				enc.Encode(row)
				os.WriteFile(filepath.Join(*out, fmt.Sprintf("hdr-%g-%d.tsv", rate, rep)), []byte(r.Percentiles), 0o644)
				fmt.Printf("%s rate=%g rep=%d tput=%.0f p50=%.2fms p99=%.2fms p99.9=%.2fms errors=%d dropped=%d\n",
					*db, rate, rep, r.Throughput, r.P50, r.P99, r.P999, r.Errors, r.Dropped)
			}
		}
	case "failover":
		rows := bench.Failover(ctx, cdb, backend, *trials, *failRate, *valueSize, *keys)
		f, err := os.Create(filepath.Join(*out, "failover.jsonl"))
		must(err)
		defer f.Close()
		enc := json.NewEncoder(f)
		for _, r := range rows {
			enc.Encode(r)
			fmt.Printf("%s kill n%d: unavailable %.0fms, restarted node caught up in %.0fms\n", *db, r.Leader+1, r.GapMS, r.CatchUpMS)
		}
	default:
		die(fmt.Errorf("unknown mode %q", *mode))
	}
}

func writeJSON(path string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(path, b, 0o644)
}

func must(err error) {
	if err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "loadgen:", err)
	os.Exit(1)
}
