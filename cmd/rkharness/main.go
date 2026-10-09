// Command rkharness runs linearizability tests (E1, E2, E5 correctness side,
// E6, E8) against raftkv or etcd. It needs root: every node runs in its own
// network namespace and faults use kill, SIGSTOP and iptables.
//
//	rkharness -db raftkv -fault partition-leader -trials 20 -out results/E1/partition-leader
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"raftkv/harness"
)

func main() {
	var (
		db         = flag.String("db", "raftkv", "raftkv or etcd")
		nodes      = flag.Int("nodes", 5, "cluster size")
		fault      = flag.String("fault", "partition-leader", "nemesis: "+strings.Join(harness.FaultNames(), ", "))
		trials     = flag.Int("trials", 1, "independent trials (fresh cluster each)")
		dur        = flag.Duration("time", 30*time.Second, "workload duration with faults")
		recover    = flag.Duration("recover", 5*time.Second, "healthy tail after faults stop")
		warmup     = flag.Duration("warmup", 3*time.Second, "healthy time before the first fault")
		interval   = flag.Duration("interval", 3*time.Second, "healthy time between faults")
		faultDur   = flag.Duration("fault-duration", 5*time.Second, "how long each fault lasts")
		clients    = flag.Int("clients", 10, "concurrent client processes")
		opTimeout  = flag.Duration("op-timeout", time.Second, "client operation timeout")
		checkTO    = flag.Duration("check-timeout", time.Minute, "linearizability checker timeout")
		keyRotate  = flag.Duration("key-rotate", 5*time.Second, "move the workload to fresh keys this often; make it longer than a fault plus recovery to read keys after a fault heals")
		activeKeys = flag.Int("active-keys", 2, "keys in use at a time")
		values     = flag.Int("values", 5, "writes pick values in [0, values)")
		unique     = flag.Bool("unique-values", false, "give every write a distinct value (CAS expects the client's last seen value)")
		bin        = flag.String("bin", "/tmp/raftkv-bin", "directory with raftkv, raftkv-bugs and etcd binaries")
		readMode   = flag.String("read-mode", "readindex", "raftkv read path: log, readindex, lease")
		bugs       = flag.String("bugs", "", "comma-separated injected bugs (uses the raftkv-bugs binary)")
		lazyfs     = flag.Bool("lazyfs", false, "raftkv loses unsynced writes on kill")
		election   = flag.Duration("election-timeout", time.Second, "election timeout (both systems)")
		heartbeat  = flag.Duration("heartbeat", 100*time.Millisecond, "heartbeat interval (both systems)")
		serial     = flag.Bool("etcd-serializable", false, "etcd: serializable (unsafe) reads, for E8")
		out        = flag.String("out", "results/scratch", "output directory")
	)
	flag.Parse()
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "rkharness needs root (network namespaces, iptables)")
		os.Exit(2)
	}
	o := harness.TestOptions{
		DB: *db, Nodes: *nodes, Fault: *fault, Duration: *dur, Recover: *recover,
		Schedule:     harness.Schedule{Warmup: *warmup, Interval: *interval, Duration: *faultDur},
		Workload:     harness.WorkloadOptions{Clients: *clients, OpTimeout: *opTimeout, KeyRotate: *keyRotate, ActiveKeys: *activeKeys, Values: *values, UniqueValues: *unique},
		CheckTimeout: *checkTO,
		RaftKV: harness.RaftKVOptions{
			Bin: filepath.Join(*bin, "raftkv"), ReadMode: *readMode, LazyFS: *lazyfs,
			ElectionTimeout: *election, Heartbeat: *heartbeat,
		},
		Etcd: harness.EtcdOptions{Bin: filepath.Join(*bin, "etcd"), Serializable: *serial, ElectionTimeout: *election, Heartbeat: *heartbeat},
		Out:  *out,
	}
	if *bugs != "" {
		o.RaftKV.Bugs = strings.Split(*bugs, ",")
		o.RaftKV.Bin = filepath.Join(*bin, "raftkv-bugs")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var results []harness.TrialResult
	for t := 1; t <= *trials; t++ {
		r := harness.RunTrial(o, t)
		results = append(results, r)
		detail := ""
		if r.Check.Result == "Illegal" {
			detail = fmt.Sprintf(" detected at %.1fs, keys %v", r.Check.DetectedAt, r.Check.BadKeys)
		}
		if r.Error != "" {
			detail = " error: " + r.Error
		}
		fmt.Printf("trial %d/%d: %s ops=%d ok=%d info=%d fail=%d faults=%d%s (%.0fs)\n",
			t, *trials, r.Check.Result, r.Check.Ops, r.Check.OK, r.Check.Info, r.Check.Fail, r.Faults, detail, r.Wall)
	}
	s := harness.Summarize(o, results)
	b := filepath.Join(*out, "summary.json")
	if err := writeSummary(b, s); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Printf("\n%s / %s: %d valid, %d illegal, %d unknown, %d errors; detection %.2f [%.2f, %.2f]",
		o.DB, o.Fault, s.Valid, s.Illegal, s.Unknown, s.Errors, s.Detection, s.DetectionLow, s.DetectionHigh)
	if s.Illegal > 0 {
		fmt.Printf("; median time to detection %.1fs", s.MedianDetectS)
	}
	fmt.Println()
}
