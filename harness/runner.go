package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type TestOptions struct {
	DB       string // raftkv or etcd
	Nodes    int
	Fault    string
	Duration time.Duration // workload length (faults included)
	Recover  time.Duration // healthy tail after the last heal
	Schedule Schedule
	Workload WorkloadOptions
	// CheckTimeout bounds each Porcupine run; "unknown" results are reported.
	CheckTimeout time.Duration

	RaftKV RaftKVOptions
	Etcd   EtcdOptions
	Out    string
}

type TrialResult struct {
	Trial  int            `json:"trial"`
	Check  CheckResult    `json:"check"`
	Faults int            `json:"faults"`
	Events []NemesisEvent `json:"events"`
	Error  string         `json:"error,omitempty"`
	Wall   float64        `json:"wall_s"`
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeJSONL[T any](path string, rows []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

// RunTrial sets up a fresh cluster, runs one test and checks it.
func RunTrial(o TestOptions, trial int) TrialResult {
	res := TrialResult{Trial: trial}
	t0 := time.Now()
	dir := filepath.Join(o.Out, fmt.Sprintf("trial-%03d", trial))
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		res.Error = err.Error()
		return res
	}
	// Node data lives outside the results tree (it can be large).
	dataDir := filepath.Join(os.TempDir(), "raftlab-data")
	os.RemoveAll(dataDir)
	defer os.RemoveAll(dataDir)

	nw, err := SetupNetwork(o.Nodes)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer TeardownNetwork(o.Nodes)

	var db DB
	switch o.DB {
	case "raftkv":
		db = NewRaftKV(o.Nodes, dataDir, o.RaftKV)
	case "etcd":
		db = NewEtcd(o.Nodes, dataDir, o.Etcd)
	default:
		res.Error = "unknown db " + o.DB
		return res
	}
	defer func() {
		db.Teardown()
		// Keep node logs with the trial's results.
		logs, _ := filepath.Glob(filepath.Join(dataDir, "n*.log"))
		for _, l := range logs {
			b, _ := os.ReadFile(l)
			os.WriteFile(filepath.Join(dir, filepath.Base(l)), b, 0o644)
		}
	}()
	if err := db.Setup(); err != nil {
		res.Error = "setup: " + err.Error()
		return res
	}

	electionTimeout := o.RaftKV.ElectionTimeout
	if o.DB == "etcd" {
		electionTimeout = o.Etcd.ElectionTimeout
	}
	e := &env{db: db, net: nw, n: o.Nodes, rng: rand.New(rand.NewSource(time.Now().UnixNano())), electionTimeout: electionTimeout}
	start := time.Now()
	faultCtx, stopFaults := context.WithTimeout(context.Background(), o.Duration)
	defer stopFaults()
	workCtx, stopWork := context.WithTimeout(context.Background(), o.Duration+o.Recover)
	defer stopWork()

	sched := o.Schedule
	sched.Fault = o.Fault
	evCh := make(chan []NemesisEvent, 1)
	go func() { evCh <- RunNemesis(faultCtx, e, sched, start) }()
	history, err := RunRegister(workCtx, db, start, o.Workload)
	res.Events = <-evCh
	for _, ev := range res.Events {
		if ev.Kind == "start" {
			res.Faults++
		}
	}
	if err != nil {
		res.Error = err.Error()
	}
	writeJSONL(filepath.Join(dir, "history.jsonl"), history)
	writeJSONL(filepath.Join(dir, "nemesis.jsonl"), res.Events)
	res.Check = Check(history, o.CheckTimeout, filepath.Join(dir, "linearizability.html"))
	if res.Check.Result != "Illegal" {
		os.Remove(filepath.Join(dir, "linearizability.html"))
	}
	res.Wall = time.Since(t0).Seconds()
	writeJSON(filepath.Join(dir, "result.json"), res)
	return res
}

// Summary aggregates trials of one configuration.
type Summary struct {
	Config        TestOptions `json:"config"`
	Trials        int         `json:"trials"`
	Valid         int         `json:"valid"`   // linearizable
	Illegal       int         `json:"illegal"` // anomaly found
	Unknown       int         `json:"unknown"` // checker timed out
	Errors        int         `json:"errors"`
	Detection     float64     `json:"detection_rate"`
	DetectionLow  float64     `json:"detection_ci95_low"`
	DetectionHigh float64     `json:"detection_ci95_high"`
	MedianDetectS float64     `json:"median_time_to_detection_s,omitempty"`
	Ops           int         `json:"ops"`
	OKOps         int         `json:"ok_ops"`
	InfoOps       int         `json:"info_ops"`
	FailOps       int         `json:"fail_ops"`
}

func Summarize(o TestOptions, rs []TrialResult) Summary {
	s := Summary{Config: o, Trials: len(rs)}
	var times []float64
	for _, r := range rs {
		switch {
		case r.Error != "":
			s.Errors++
		case r.Check.Result == "Ok":
			s.Valid++
		case r.Check.Result == "Illegal":
			s.Illegal++
			times = append(times, r.Check.DetectedAt)
		default:
			s.Unknown++
		}
		s.Ops += r.Check.Ops
		s.OKOps += r.Check.OK
		s.InfoOps += r.Check.Info
		s.FailOps += r.Check.Fail
	}
	n := s.Valid + s.Illegal
	s.Detection = ratio(s.Illegal, n)
	s.DetectionLow, s.DetectionHigh = Wilson(s.Illegal, n, 1.96)
	if len(times) > 0 {
		sort.Float64s(times)
		m := len(times) / 2
		if len(times)%2 == 1 {
			s.MedianDetectS = times[m]
		} else {
			s.MedianDetectS = (times[m-1] + times[m]) / 2
		}
	}
	return s
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Wilson returns the Wilson score interval for k successes in n trials.
func Wilson(k, n int, z float64) (float64, float64) {
	if n == 0 {
		return 0, 1
	}
	p := float64(k) / float64(n)
	nf := float64(n)
	den := 1 + z*z/nf
	centre := (p + z*z/(2*nf)) / den
	half := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / den
	return math.Max(0, centre-half), math.Min(1, centre+half)
}
