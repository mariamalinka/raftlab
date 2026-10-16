package harness

// Offline analysis (not in the repo): for a key whose reduced check is
// Illegal, find the earliest Illegal prefix of the reduced history (100 ms
// steps), then try adding back each dropped indeterminate op one at a time
// to see whether a single dropped op explains the prefix.
import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
)

func TestExplain(t *testing.T) {
	p := os.Getenv("EXPLAIN_HIST")
	if p == "" {
		t.Skip()
	}
	key, _ := strconv.Atoi(os.Getenv("EXPLAIN_KEY"))
	f, _ := os.Open(p)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var ops []Op
	for sc.Scan() {
		var o Op
		json.Unmarshal(sc.Bytes(), &o)
		if o.Key == key {
			ops = append(ops, o)
		}
	}
	kept, _ := reduceKey(ops)
	keptSet := map[int]bool{}
	for i := range kept {
		_ = i
	}
	var dropped []Op
	for _, o := range ops {
		if o.Type == "info" && o.F != "read" {
			found := false
			for _, k := range kept {
				if k == o {
					found = true
					break
				}
			}
			if !found {
				dropped = append(dropped, o)
			}
		}
	}
	_ = keptSet
	to := 30 * time.Second
	chk := func(h []Op, cut int64) porcupine.CheckResult {
		return porcupine.CheckOperationsTimeout(RegisterModel, toOperations(h, cut), to)
	}
	var end int64
	for _, o := range ops {
		end = max(end, o.Return)
	}
	lo, hi := int64(0), end/int64(100*time.Millisecond)+1
	for lo < hi {
		mid := (lo + hi) / 2
		if chk(kept, mid*int64(100*time.Millisecond)) == porcupine.Illegal {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	cut := lo * int64(100*time.Millisecond)
	fmt.Printf("key %d: %d ops, %d dropped; reduced history first Illegal at prefix %.1fs\n", key, len(ops), len(dropped), float64(cut)/1e9)
	for _, d := range dropped {
		if d.Call >= cut {
			continue
		}
		h := append(append([]Op{}, kept...), d)
		r := chk(h, cut)
		if r == porcupine.Ok {
			fmt.Printf("  adding back dropped %s value=%d expected=%d call=%.3fs (process %d, err %q) makes the prefix Ok\n", d.F, d.Value, d.Expected, float64(d.Call)/1e9, d.Process, d.Err)
		}
	}
	// ops of this key completing in the 300 ms before the cut, for context
	for _, o := range ops {
		if o.Return > cut-300*int64(time.Millisecond) && o.Call < cut && o.Type != "fail" {
			fmt.Printf("    ctx %s %s v=%d exp=%d casok=%v call=%.3f ret=%.3f p%d\n", o.Type, o.F, o.Value, o.Expected, o.CASOK, float64(o.Call)/1e9, float64(o.Return)/1e9, o.Process)
		}
	}
}
