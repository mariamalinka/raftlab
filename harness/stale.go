package harness

import (
	"math"
	"sort"
)

// StaleReads is the result of the conservative stale-read scan.
type StaleReads struct {
	Reads int   `json:"reads"`          // reads flagged
	Keys  []int `json:"keys,omitempty"` // keys with a flagged read
}

// ScanStaleReads flags reads whose value no write could have produced, the
// same rule as analysis/stale_reads.py. For a read R, the candidate writes
// are the writes (ok writes, ok successful CASes, indeterminate writes and
// CASes) invoked before R returned that are not superseded: no other write
// was invoked after them and returned before R was invoked. The initial
// value (-1) is a candidate until some write has returned. A flagged read is
// definitely not linearizable; an unflagged one may still be. It is a
// separate metric from the Porcupine result and never changes it.
func ScanStaleReads(history []Op) StaleReads {
	byKey := map[int][]Op{}
	for _, o := range history {
		if o.Type != "fail" {
			byKey[o.Key] = append(byKey[o.Key], o)
		}
	}
	keys := make([]int, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var res StaleReads
	for _, k := range keys {
		ops := byKey[k]
		var writes []Op
		for _, o := range ops {
			if o.F == "write" || (o.F == "cas" && (o.Type == "info" || o.CASOK)) {
				writes = append(writes, o)
			}
		}
		ret := func(o Op) int64 {
			if o.Type == "ok" {
				return o.Return
			}
			return math.MaxInt64
		}
		n := 0
		for _, r := range ops {
			if r.F != "read" || r.Type != "ok" {
				continue
			}
			c, done := int64(-1), false
			for _, w := range writes {
				if ret(w) < r.Call {
					done = true
					c = max(c, w.Call)
				}
			}
			ok := !done && r.Value == -1
			for _, w := range writes {
				if ok {
					break
				}
				ok = w.Call <= r.Return && ret(w) >= c && w.Value == r.Value
			}
			if !ok {
				n++
			}
		}
		if n > 0 {
			res.Reads += n
			res.Keys = append(res.Keys, k)
		}
	}
	return res
}
