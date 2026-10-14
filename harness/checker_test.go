package harness

import (
	"math"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
)

// Tests for the checker's outcomes: plain Ok and Illegal must behave exactly
// as before, the reduction may only turn an Unknown into "Ok (reduced)", and
// it must never call a non-linearizable history Ok.

func w(p, key, v int, call, ret int64) Op {
	return Op{Process: p, F: "write", Key: key, Value: v, Type: "ok", Call: call, Return: ret}
}

func rd(p, key, v int, call, ret int64) Op {
	return Op{Process: p, F: "read", Key: key, Value: v, Type: "ok", Call: call, Return: ret}
}

func infoW(p, key, v int, call int64) Op {
	return Op{Process: p, F: "write", Key: key, Value: v, Type: "info", Call: call, Return: call + 1000}
}

// forceUnknown makes the full-history check time out, as on the large
// histories that need the reduction, and restores it when the test ends.
func forceUnknown(t *testing.T) {
	t.Helper()
	saved := fullCheck
	fullCheck = func(porcupine.Model, []porcupine.Operation, time.Duration) porcupine.CheckResult {
		return porcupine.Unknown
	}
	t.Cleanup(func() { fullCheck = saved })
}

func TestCheckOkUnchanged(t *testing.T) {
	h := []Op{w(0, 1, 10, 0, 10), rd(1, 1, 10, 20, 30), infoW(2, 1, 11, 40)}
	r := Check(h, 10*time.Second, "")
	if r.Result != ResultOk {
		t.Fatalf("result %q, want %q", r.Result, ResultOk)
	}
	if r.ReducedKeys != nil || r.UnknownKeys != nil || r.DroppedOps != 0 {
		t.Fatalf("the reduction ran on a history the full check decided: %+v", r)
	}
}

func TestCheckIllegalUnchanged(t *testing.T) {
	h := []Op{w(0, 1, 10, 0, 10), w(0, 1, 20, 20, 30), rd(1, 1, 10, 40, 50), w(0, 2, 5, 0, 10)}
	r := Check(h, 10*time.Second, "")
	if r.Result != ResultIllegal {
		t.Fatalf("result %q, want %q", r.Result, ResultIllegal)
	}
	if !slices.Equal(r.BadKeys, []int{1}) || r.DetectedAt <= 0 {
		t.Fatalf("bad keys %v, detected at %.1fs: want key 1 and a detection time", r.BadKeys, r.DetectedAt)
	}
	if r.ReducedKeys != nil || r.DroppedOps != 0 {
		t.Fatalf("the reduction ran on an Illegal history: %+v", r)
	}
}

func TestUnknownResolvedByReduction(t *testing.T) {
	forceUnknown(t)
	h := []Op{
		w(0, 1, 10, 0, 10), infoW(1, 1, 99, 5), rd(2, 1, 10, 20, 30), // 99 is never read
		w(0, 2, 7, 0, 10), rd(1, 2, 7, 20, 30),
	}
	r := Check(h, 10*time.Second, "")
	if r.Result != ResultOkReduced {
		t.Fatalf("result %q, want %q", r.Result, ResultOkReduced)
	}
	if !slices.Equal(r.ReducedKeys, []int{1}) || r.DroppedOps != 1 || r.UnknownKeys != nil {
		t.Fatalf("got reduced keys %v, dropped %d, unknown keys %v; want [1], 1, none",
			r.ReducedKeys, r.DroppedOps, r.UnknownKeys)
	}
}

// A reduced history that is not Ok is not a proof of anything: the trial
// stays Unknown, neither Ok nor Illegal.
func TestUnknownStaysUnknown(t *testing.T) {
	forceUnknown(t)
	h := []Op{
		w(0, 1, 10, 0, 10), w(0, 1, 20, 20, 30), rd(1, 1, 10, 40, 50), infoW(2, 1, 99, 5),
		w(0, 2, 7, 0, 10), rd(1, 2, 7, 20, 30),
	}
	r := Check(h, 10*time.Second, "")
	if r.Result != ResultUnknown {
		t.Fatalf("result %q, want %q", r.Result, ResultUnknown)
	}
	if !slices.Equal(r.UnknownKeys, []int{1}) {
		t.Fatalf("unknown keys %v, want [1]", r.UnknownKeys)
	}
}

// Soundness: on random small histories with indeterminate operations,
// whenever every key's reduced history is linearizable, so is the full
// history. Equivalently, the reduction never turns an Illegal into Ok.
func TestReductionNeverHidesViolations(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var bothOk, illegal, dropped int
	for i := 0; i < 3000; i++ {
		h := randomHistory(rng)
		full := porcupine.CheckOperations(RegisterModel, toOperations(h, math.MaxInt64))
		reducedOk := true
		for _, part := range byKey(h) {
			kept, d := reduceKey(part)
			dropped += d
			if !porcupine.CheckOperations(RegisterModel, toOperations(kept, math.MaxInt64)) {
				reducedOk = false
			}
		}
		if reducedOk && !full {
			t.Fatalf("history %d: reduced history is linearizable but the full one is not:\n%+v", i, h)
		}
		if reducedOk {
			bothOk++
		}
		if !full {
			illegal++
		}
	}
	// The property is only meaningful if both outcomes and dropped
	// operations actually occur.
	if bothOk < 100 || illegal < 100 || dropped < 100 {
		t.Fatalf("weak sample: %d reduced-Ok, %d illegal, %d dropped ops", bothOk, illegal, dropped)
	}
	t.Logf("3000 histories: %d reduced-Ok (all Ok in full), %d illegal, %d ops dropped", bothOk, illegal, dropped)
}

// randomHistory builds a short concurrent history on two keys by running
// the operations against a real register at a random point inside each
// one's interval, so it starts out linearizable. Indeterminate writes and
// CASes take effect or not at random. Then, in about a third of histories,
// one read is given a different earlier value, which usually makes the
// history non-linearizable.
func randomHistory(rng *rand.Rand) []Op {
	type ev struct {
		at int64
		i  int
	}
	var h []Op
	var evs []ev
	next := 1
	for p := 0; p < 3; p++ {
		t := int64(rng.Intn(5))
		for j := 0; j < 4+rng.Intn(3); j++ {
			call := t
			ret := call + 1 + int64(rng.Intn(15))
			o := Op{Process: p, Key: rng.Intn(2), Call: call, Return: ret, Type: "ok"}
			switch rng.Intn(4) {
			case 0, 1:
				o.F, o.Value = "write", next
				next++
			case 2:
				o.F, o.Value = "cas", next
				next++
			default:
				o.F = "read"
			}
			if o.F != "read" && rng.Intn(4) == 0 {
				o.Type = "info"
			}
			evs = append(evs, ev{call + rng.Int63n(ret-call+1), len(h)})
			h = append(h, o)
			t = ret + int64(rng.Intn(3))
		}
	}
	slices.SortFunc(evs, func(a, b ev) int { return int(a.at - b.at) })
	reg := map[int]int{0: -1, 1: -1}
	var seen []int
	for _, e := range evs {
		o := &h[e.i]
		effect := o.Type == "ok" || rng.Intn(2) == 0
		switch o.F {
		case "write":
			if effect {
				reg[o.Key] = o.Value
			}
		case "cas":
			// Expect the current value half the time, an older one otherwise.
			o.Expected = reg[o.Key]
			if len(seen) > 0 && rng.Intn(2) == 0 {
				o.Expected = seen[rng.Intn(len(seen))]
			}
			o.CASOK = o.Expected == reg[o.Key]
			if effect && o.CASOK {
				reg[o.Key] = o.Value
			}
		case "read":
			o.Value = reg[o.Key]
		}
		seen = append(seen, reg[o.Key])
	}
	if rng.Intn(3) == 0 {
		for _, i := range rng.Perm(len(h)) {
			if h[i].F == "read" && len(seen) > 0 {
				h[i].Value = seen[rng.Intn(len(seen))]
				break
			}
		}
	}
	return h
}

func byKey(h []Op) [][]Op {
	m := map[int][]Op{}
	for _, o := range h {
		m[o.Key] = append(m[o.Key], o)
	}
	var out [][]Op
	for _, k := range []int{0, 1} {
		if len(m[k]) > 0 {
			out = append(out, m[k])
		}
	}
	return out
}

func TestSummarizeKeepsOutcomesApart(t *testing.T) {
	rs := []TrialResult{
		{Check: CheckResult{Result: ResultOk}},
		{Check: CheckResult{Result: ResultOkReduced}},
		{Check: CheckResult{Result: ResultUnknown}},
		{Check: CheckResult{Result: ResultUnknown}},
		{Check: CheckResult{Result: ResultIllegal}},
		{Error: "setup failed"},
	}
	s := Summarize(TestOptions{}, rs)
	if s.Valid != 1 || s.ValidReduced != 1 || s.Unknown != 2 || s.Illegal != 1 || s.Errors != 1 {
		t.Fatalf("valid %d, valid_reduced %d, unknown %d, illegal %d, errors %d; want 1, 1, 2, 1, 1",
			s.Valid, s.ValidReduced, s.Unknown, s.Illegal, s.Errors)
	}
	// Unknown trials are not in the denominator: 1 illegal out of 3 decided.
	if s.Detection != 1.0/3 {
		t.Fatalf("detection rate %v, want 1/3", s.Detection)
	}
}

func TestScanStaleReads(t *testing.T) {
	cases := []struct {
		name string
		h    []Op
		want int
	}{
		{"fresh read", []Op{w(0, 1, 10, 0, 10), rd(1, 1, 10, 20, 30)}, 0},
		{"stale read", []Op{w(0, 1, 10, 0, 10), w(0, 1, 20, 20, 30), rd(1, 1, 10, 40, 50)}, 1},
		{"concurrent write may explain it", []Op{w(0, 1, 10, 0, 10), w(0, 1, 20, 20, 60), rd(1, 1, 10, 40, 50)}, 0},
		{"indeterminate write may explain it", []Op{w(0, 1, 10, 0, 10), infoW(2, 1, 20, 15), rd(1, 1, 20, 40, 50)}, 0},
		{"initial value before any write returned", []Op{w(0, 1, 10, 0, 50), rd(1, 1, -1, 10, 20)}, 0},
		{"initial value after a write returned", []Op{w(0, 1, 10, 0, 10), rd(1, 1, -1, 20, 30)}, 1},
		{"failed write is ignored", []Op{w(0, 1, 10, 0, 10), {F: "write", Key: 1, Value: 20, Type: "fail", Call: 15, Return: 18}, rd(1, 1, 20, 40, 50)}, 1},
	}
	for _, c := range cases {
		if got := ScanStaleReads(c.h).Reads; got != c.want {
			t.Errorf("%s: %d stale reads, want %d", c.name, got, c.want)
		}
	}
}

func TestPlannedFaults(t *testing.T) {
	s := Schedule{Fault: "kill-all", Warmup: 3 * time.Second, Interval: 3 * time.Second, Duration: 5 * time.Second}
	for _, c := range []struct {
		d    time.Duration
		want int
	}{{20 * time.Second, 2}, {24 * time.Second, 3}, {25 * time.Second, 3}, {30 * time.Second, 3}, {7 * time.Second, 0}} {
		if got := PlannedFaults(s, c.d); got != c.want {
			t.Errorf("run of %v: %d planned faults, want %d", c.d, got, c.want)
		}
	}
	if got := PlannedFaults(Schedule{Fault: "none", Duration: time.Second}, time.Minute); got != 0 {
		t.Errorf("fault none: %d planned, want 0", got)
	}
}
