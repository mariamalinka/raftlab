package harness

// Offline analysis (not in the repo). Sound Ok proof by partial reduction:
// start from the reduced key history; while it is Illegal, find the
// earliest Illegal prefix and add back one dropped indeterminate op that
// makes that prefix Ok; repeat. Dropping any indeterminate op is sound for
// Ok (it may be linearized as having no effect), so a final Ok on a history
// that still omits some dropped ops proves the full key history Ok. A final
// Illegal or Unknown proves nothing.
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

func TestGreedy(t *testing.T) {
	p := os.Getenv("GREEDY_HIST")
	if p == "" {
		t.Skip()
	}
	key, _ := strconv.Atoi(os.Getenv("GREEDY_KEY"))
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
	in := map[Op]bool{}
	for _, k := range kept {
		in[k] = true
	}
	var dropped []Op
	for _, o := range ops {
		if !in[o] {
			dropped = append(dropped, o)
		}
	}
	to := 60 * time.Second
	chk := func(h []Op, cut int64) porcupine.CheckResult {
		return porcupine.CheckOperationsTimeout(RegisterModel, toOperations(h, cut), to)
	}
	var end int64
	for _, o := range ops {
		end = max(end, o.Return)
	}
	added := 0
	for iter := 0; iter < len(dropped)+1; iter++ {
		r := chk(kept, 1<<62)
		if r != porcupine.Illegal {
			fmt.Printf("key %d: %d ops, %d dropped by reduceKey, %d added back, final check %s\n", key, len(ops), len(dropped), added, r)
			return
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
		fixed := false
		for i := len(dropped) - 1; i >= 0; i-- { // prefer the latest one before the cut
			d := dropped[i]
			if in[d] || d.Call >= cut {
				continue
			}
			h := append(append([]Op{}, kept...), d)
			if chk(h, cut) == porcupine.Ok {
				kept, in[d] = h, true
				added++
				fixed = true
				break
			}
		}
		if !fixed {
			fmt.Printf("key %d: no single dropped op fixes the prefix at %.1fs (%d added so far); inconclusive\n", key, float64(cut)/1e9, added)
			return
		}
	}
}
