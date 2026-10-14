package harness

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/anishathalye/porcupine"
)

// Register model, partitioned by key. A key's state is its value (-1 when
// missing). Indeterminate writes and CASes may or may not have happened, so
// the model is nondeterministic for them.

type regInput struct {
	F        string
	Key      int
	Value    int
	Expected int
}

type regOutput struct {
	Value   int  // read result
	OK      bool // CAS succeeded
	Unknown bool // indeterminate
}

var registerModel = porcupine.NondeterministicModel{
	Partition: func(history []porcupine.Operation) [][]porcupine.Operation {
		byKey := map[int][]porcupine.Operation{}
		var keys []int
		for _, op := range history {
			k := op.Input.(regInput).Key
			if _, ok := byKey[k]; !ok {
				keys = append(keys, k)
			}
			byKey[k] = append(byKey[k], op)
		}
		sort.Ints(keys)
		var out [][]porcupine.Operation
		for _, k := range keys {
			out = append(out, byKey[k])
		}
		return out
	},
	Init: func() []any { return []any{-1} },
	Step: func(state, input, output any) []any {
		s, in, out := state.(int), input.(regInput), output.(regOutput)
		switch in.F {
		case "read":
			if out.Value == s {
				return []any{s}
			}
			return nil
		case "write":
			if out.Unknown {
				return []any{s, in.Value}
			}
			return []any{in.Value}
		case "cas":
			if out.Unknown {
				if s == in.Expected {
					return []any{s, in.Value}
				}
				return []any{s}
			}
			if out.OK {
				if s == in.Expected {
					return []any{in.Value}
				}
				return nil
			}
			if s != in.Expected {
				return []any{s}
			}
			return nil
		}
		return nil
	},
	DescribeOperation: func(input, output any) string {
		in, out := input.(regInput), output.(regOutput)
		switch in.F {
		case "read":
			return fmt.Sprintf("read(r%d) -> %d", in.Key, out.Value)
		case "write":
			if out.Unknown {
				return fmt.Sprintf("write(r%d, %d) ?", in.Key, in.Value)
			}
			return fmt.Sprintf("write(r%d, %d)", in.Key, in.Value)
		default:
			res := fmt.Sprint(out.OK)
			if out.Unknown {
				res = "?"
			}
			return fmt.Sprintf("cas(r%d, %d->%d) %s", in.Key, in.Expected, in.Value, res)
		}
	},
	DescribeState: func(state any) string { return fmt.Sprint(state) },
}

var RegisterModel = registerModel.ToModel()

// toOperations converts a history, keeping only operations invoked before
// cutoff (ns) and treating any still running at cutoff as indeterminate.
// Failed operations never happened, so they are dropped, as are
// indeterminate reads (they have no effect).
func toOperations(history []Op, cutoff int64) []porcupine.Operation {
	var ops []porcupine.Operation
	for _, h := range history {
		if h.Call > cutoff {
			continue
		}
		typ := h.Type
		if h.Return > cutoff && typ == "ok" {
			typ = "info"
		}
		if typ == "fail" || (typ == "info" && h.F == "read") {
			continue
		}
		in := regInput{F: h.F, Key: h.Key, Value: h.Value, Expected: h.Expected}
		out := regOutput{OK: h.CASOK}
		ret := h.Return
		if h.F == "read" {
			out.Value = h.Value
		}
		if typ == "info" {
			out.Unknown = true
			ret = math.MaxInt64
		}
		ops = append(ops, porcupine.Operation{ClientId: h.Process, Input: in, Call: h.Call, Output: out, Return: ret})
	}
	return ops
}

// Check outcomes. "Ok (reduced)" is a proven pass reached only after the
// full check returned Unknown; see reduce.
const (
	ResultOk        = "Ok"
	ResultOkReduced = "Ok (reduced)"
	ResultUnknown   = "Unknown"
	ResultIllegal   = "Illegal"
)

type CheckResult struct {
	Result string `json:"result"` // Ok, Ok (reduced), Illegal, or Unknown (checker timed out)
	// DetectedAt is the shortest history prefix (seconds) already shown to
	// be non-linearizable; set only when Result is illegal.
	DetectedAt float64 `json:"detected_at_s,omitempty"`
	Ops        int     `json:"ops"`
	OK         int     `json:"ok"`
	Fail       int     `json:"fail"`
	Info       int     `json:"info"`
	BadKeys    []int   `json:"bad_keys,omitempty"`
	// Set only when the full check was Unknown and the per-key reduced
	// check ran: keys proven by it, keys it could not decide, and how many
	// indeterminate operations it dropped.
	ReducedKeys []int `json:"reduced_keys,omitempty"`
	UnknownKeys []int `json:"unknown_keys,omitempty"`
	DroppedOps  int   `json:"dropped_ops,omitempty"`
}

// fullCheck checks the whole history; tests replace it to force a timeout.
var fullCheck = porcupine.CheckOperationsTimeout

// Check verifies the full history and, if it is not linearizable, finds how
// early the anomaly was detectable. vizPath, if set, receives Porcupine's
// HTML visualization of a failing history.
func Check(history []Op, timeout time.Duration, vizPath string) CheckResult {
	var r CheckResult
	var end int64
	for _, h := range history {
		r.Ops++
		switch h.Type {
		case "ok":
			r.OK++
		case "fail":
			r.Fail++
		case "info":
			r.Info++
		}
		end = max(end, h.Return)
	}
	all := toOperations(history, math.MaxInt64)
	res := fullCheck(RegisterModel, all, timeout)
	r.Result = string(res)
	if res == porcupine.Unknown {
		r.reduce(history, timeout)
	}
	if res != porcupine.Illegal {
		return r
	}
	// Narrow down to the keys that fail; the rest of the work only looks
	// at their operations, which keeps it fast.
	r.BadKeys = badKeys(all, timeout)
	bad := map[int]bool{}
	for _, k := range r.BadKeys {
		bad[k] = true
	}
	var badHist []Op
	for _, h := range history {
		if bad[h.Key] {
			badHist = append(badHist, h)
		}
	}
	if vizPath != "" {
		_, info := porcupine.CheckOperationsVerbose(RegisterModel, toOperations(badHist, math.MaxInt64), timeout)
		porcupine.VisualizePath(RegisterModel, info, vizPath)
	}
	// Binary search for the earliest illegal prefix, at 100 ms resolution.
	lo, hi := int64(0), end/int64(100*time.Millisecond)+1
	for lo < hi {
		mid := (lo + hi) / 2
		cut := mid * int64(100*time.Millisecond)
		if porcupine.CheckOperationsTimeout(RegisterModel, toOperations(badHist, cut), timeout) == porcupine.Illegal {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	r.DetectedAt = float64(lo) / 10
	return r
}

// badKeys re-checks each key on its own and lists those that fail.
func badKeys(ops []porcupine.Operation, timeout time.Duration) []int {
	var keys []int
	for _, part := range RegisterModel.Partition(ops) {
		if porcupine.CheckOperationsTimeout(RegisterModel, part, timeout) == porcupine.Illegal {
			keys = append(keys, part[0].Input.(regInput).Key)
		}
	}
	return keys
}

// reduce re-checks each key of a history whose full check was Unknown, after
// dropping the indeterminate writes and CASes whose value is never observed
// (reduceKey). If every key then checks Ok, the result is "Ok (reduced)";
// otherwise it stays Unknown. A reduced check that is not Ok proves nothing
// and never makes the result Illegal.
//
// Sound for Ok: under RegisterModel an indeterminate operation may always be
// linearized as having had no effect (Step returns the unchanged state among
// its outcomes). So a linearization of the reduced history extends to the
// original one by placing each dropped operation anywhere in its interval
// with no effect. Keys are independent registers, so per-key Ok is overall Ok.
func (r *CheckResult) reduce(history []Op, timeout time.Duration) {
	byKey := map[int][]Op{}
	var keys []int
	for _, h := range history {
		if _, ok := byKey[h.Key]; !ok {
			keys = append(keys, h.Key)
		}
		byKey[h.Key] = append(byKey[h.Key], h)
	}
	sort.Ints(keys)
	for _, k := range keys {
		kept, dropped := reduceKey(byKey[k])
		if porcupine.CheckOperationsTimeout(RegisterModel, toOperations(kept, math.MaxInt64), timeout) != porcupine.Ok {
			r.UnknownKeys = append(r.UnknownKeys, k)
			continue
		}
		if dropped > 0 {
			r.ReducedKeys = append(r.ReducedKeys, k)
			r.DroppedOps += dropped
		}
	}
	if len(r.UnknownKeys) == 0 {
		r.Result = ResultOkReduced
	}
}

// reduceKey drops, from one key's operations, the indeterminate writes and
// CASes whose value no completed read returned and no completed CAS expected.
// Such operations are what make Porcupine's search blow up on long
// unique-value histories (docs/VERIFICATION-2026-10-11.md, section 1).
func reduceKey(ops []Op) (kept []Op, dropped int) {
	seen := map[int]bool{}
	for _, o := range ops {
		if o.Type == "ok" && o.F == "read" {
			seen[o.Value] = true
		}
		if o.Type == "ok" && o.F == "cas" {
			seen[o.Expected] = true
		}
	}
	for _, o := range ops {
		if o.Type == "info" && o.F != "read" && !seen[o.Value] {
			dropped++
			continue
		}
		kept = append(kept, o)
	}
	return kept, dropped
}
