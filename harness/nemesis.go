package harness

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// A fault is injected by start and undone by the returned heal function.
type fault func(ctx context.Context, env *env) (heal func() error, desc string, err error)

type env struct {
	db  DB
	net *Network
	n   int
	rng *rand.Rand
	// electionTimeout lets clock faults scale with the cluster's timing.
	electionTimeout time.Duration
}

func (e *env) leader(ctx context.Context) int {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	l, err := e.db.Leader(c)
	if err != nil {
		return e.rng.Intn(e.n)
	}
	return l
}

func (e *env) shuffled() []int {
	p := e.rng.Perm(e.n)
	return p
}

func killNodes(e *env, nodes []int, desc string) (func() error, string, error) {
	for _, i := range nodes {
		if err := e.db.Kill(i); err != nil {
			return nil, "", err
		}
	}
	heal := func() error {
		for _, i := range nodes {
			if err := e.db.Start(i); err != nil {
				return err
			}
		}
		return nil
	}
	return heal, fmt.Sprintf("%s %v", desc, human(nodes)), nil
}

func pauseNodes(e *env, nodes []int, desc string) (func() error, string, error) {
	for _, i := range nodes {
		if err := e.db.Pause(i); err != nil {
			return nil, "", err
		}
	}
	heal := func() error {
		for _, i := range nodes {
			if err := e.db.Resume(i); err != nil {
				return err
			}
		}
		return nil
	}
	return heal, fmt.Sprintf("%s %v", desc, human(nodes)), nil
}

func human(nodes []int) []int {
	out := make([]int, len(nodes))
	for i, n := range nodes {
		out[i] = n + 1
	}
	sort.Ints(out)
	return out
}

func partition(e *env, groups [][]int, desc string) (func() error, string, error) {
	if err := e.net.Partition(groups); err != nil {
		return nil, "", err
	}
	var parts []string
	for _, g := range groups {
		parts = append(parts, fmt.Sprint(human(g)))
	}
	return e.net.Heal, desc + " " + strings.Join(parts, " | "), nil
}

var faults = map[string]fault{
	"kill-random": func(ctx context.Context, e *env) (func() error, string, error) {
		return killNodes(e, []int{e.rng.Intn(e.n)}, "kill")
	},
	"kill-leader": func(ctx context.Context, e *env) (func() error, string, error) {
		return killNodes(e, []int{e.leader(ctx)}, "kill leader")
	},
	"kill-majority": func(ctx context.Context, e *env) (func() error, string, error) {
		return killNodes(e, e.shuffled()[:e.n/2+1], "kill majority")
	},
	"kill-all": func(ctx context.Context, e *env) (func() error, string, error) {
		return killNodes(e, e.shuffled(), "kill all")
	},
	"pause-random": func(ctx context.Context, e *env) (func() error, string, error) {
		return pauseNodes(e, []int{e.rng.Intn(e.n)}, "pause")
	},
	"pause-leader": func(ctx context.Context, e *env) (func() error, string, error) {
		return pauseNodes(e, []int{e.leader(ctx)}, "pause leader")
	},
	"partition-leader": func(ctx context.Context, e *env) (func() error, string, error) {
		l := e.leader(ctx)
		var rest []int
		for i := range e.n {
			if i != l {
				rest = append(rest, i)
			}
		}
		return partition(e, [][]int{{l}, rest}, "isolate leader")
	},
	"partition-halves": func(ctx context.Context, e *env) (func() error, string, error) {
		p := e.shuffled()
		k := e.n / 2
		return partition(e, [][]int{p[:k], p[k:]}, "halves")
	},
	"partition-bridge": func(ctx context.Context, e *env) (func() error, string, error) {
		// Two halves that can't see each other, plus one node that sees both.
		p := e.shuffled()
		k := (e.n - 1) / 2
		a, b := p[:k], p[k+1:]
		var pairs [][2]int
		for _, x := range a {
			for _, y := range b {
				pairs = append(pairs, [2]int{x, y})
			}
		}
		if err := e.net.Heal(); err != nil {
			return nil, "", err
		}
		if err := e.net.Cut(pairs); err != nil {
			return nil, "", err
		}
		return e.net.Heal, fmt.Sprintf("bridge %v <-%d-> %v", human(a), p[k]+1, human(b)), nil
	},
	"clock-skew": func(ctx context.Context, e *env) (func() error, string, error) {
		// Random jumps of up to ±2 election timeouts and rates of 0.25x-4x on
		// a random subset of nodes.
		var desc []string
		for _, i := range e.shuffled()[:1+e.rng.Intn(e.n)] {
			jump := time.Duration((e.rng.Float64()*4 - 2) * float64(e.electionTimeout))
			rate := []float64{0.25, 0.5, 1, 2, 4}[e.rng.Intn(5)]
			if err := e.db.SkewClock(i, jump, rate); err != nil {
				return nil, "", err
			}
			desc = append(desc, fmt.Sprintf("n%d %+v x%.2f", i+1, jump.Round(time.Millisecond), rate))
		}
		return e.db.ResetClocks, "clock " + strings.Join(desc, ", "), nil
	},
	// Slow the leader's clock, then isolate it, so its lease outlives the
	// real election timeout while the majority elects a new leader. The
	// strong version (x0.25) exceeds any sane drift bound; the mild one
	// (x0.92) stays inside the default 10% bound and only breaks leases
	// that ignore drift.
	"clock-partition":      clockPartition(0.25),
	"clock-partition-mild": clockPartition(0.92),
}

func clockPartition(rate float64) fault {
	return func(ctx context.Context, e *env) (func() error, string, error) {
		l := e.leader(ctx)
		if err := e.db.SkewClock(l, 0, rate); err != nil {
			return nil, "", err
		}
		var rest []int
		for i := range e.n {
			if i != l {
				rest = append(rest, i)
			}
		}
		heal, desc, err := partition(e, [][]int{{l}, rest}, fmt.Sprintf("clock x%.2f on n%d +", rate, l+1))
		if err != nil {
			return nil, "", err
		}
		return func() error {
			if err := heal(); err != nil {
				return err
			}
			return e.db.ResetClocks()
		}, desc, nil
	}
}

// FaultNames lists the nemeses, plus "none" and "mixed".
func FaultNames() []string {
	var names []string
	for k := range faults {
		names = append(names, k)
	}
	sort.Strings(names)
	return append(names, "mixed", "none")
}

// mixed picks any process, partition or clock fault each round (H2's
// combined schedule).
var mixedPool = []string{"kill-random", "kill-leader", "pause-random", "pause-leader",
	"partition-leader", "partition-halves", "partition-bridge", "clock-partition"}

// NemesisEvent is logged for every fault start and heal.
type NemesisEvent struct {
	Time  int64  `json:"time"` // ns since start
	Kind  string `json:"kind"` // start, heal, error
	Fault string `json:"fault"`
	Desc  string `json:"desc"`
}

type Schedule struct {
	Fault    string
	Warmup   time.Duration // healthy time before the first fault
	Interval time.Duration // healthy time between faults
	Duration time.Duration // how long each fault lasts
}

// RunNemesis injects faults on schedule until ctx ends, then heals.
func RunNemesis(ctx context.Context, e *env, s Schedule, start time.Time) []NemesisEvent {
	var events []NemesisEvent
	log := func(kind, f, desc string) {
		events = append(events, NemesisEvent{Time: int64(time.Since(start)), Kind: kind, Fault: f, Desc: desc})
	}
	if s.Fault == "none" {
		<-ctx.Done()
		return events
	}
	sleep := func(d time.Duration) bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(d):
			return true
		}
	}
	if !sleep(s.Warmup) {
		return events
	}
	for {
		name := s.Fault
		if name == "mixed" {
			name = mixedPool[e.rng.Intn(len(mixedPool))]
		}
		f := faults[name]
		heal, desc, err := f(ctx, e)
		if err != nil {
			log("error", name, err.Error())
		} else {
			log("start", name, desc)
			sleep(s.Duration)
			if err := heal(); err != nil {
				log("error", name, "heal: "+err.Error())
			} else {
				log("heal", name, desc)
			}
		}
		if !sleep(s.Interval) {
			return events
		}
	}
}
