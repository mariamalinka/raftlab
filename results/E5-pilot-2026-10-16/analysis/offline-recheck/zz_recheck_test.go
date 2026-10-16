package harness

// Offline per-key re-check of saved histories (E1/E2 analysis, 2026-10-15).
// Not part of the repository. RECHECK_HIST=path RECHECK_KEYS=all|k1,k2
// RECHECK_TIMEOUT=30m. For each key: full check, then the same sound
// reduction the harness uses (reduceKey) if the full check is Unknown.
import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
)

func TestRecheck(t *testing.T) {
	p := os.Getenv("RECHECK_HIST")
	if p == "" {
		t.Skip()
	}
	to, _ := time.ParseDuration(os.Getenv("RECHECK_TIMEOUT"))
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	var hist []Op
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var o Op
		if err := json.Unmarshal(sc.Bytes(), &o); err != nil {
			t.Fatal(err)
		}
		hist = append(hist, o)
	}
	byKey := map[int][]Op{}
	for _, h := range hist {
		byKey[h.Key] = append(byKey[h.Key], h)
	}
	var keys []int
	if ks := os.Getenv("RECHECK_KEYS"); ks != "" && ks != "all" {
		for _, s := range strings.Split(ks, ",") {
			k, _ := strconv.Atoi(s)
			keys = append(keys, k)
		}
	} else {
		for k := range byKey {
			keys = append(keys, k)
		}
		sort.Ints(keys)
	}
	cut := int64(math.MaxInt64)
	if c := os.Getenv("RECHECK_CUT_NS"); c != "" {
		cut, _ = strconv.ParseInt(c, 10, 64)
	}
	for _, k := range keys {
		ops := byKey[k]
		t0 := time.Now()
		full := porcupine.Unknown
		if os.Getenv("RECHECK_MODE") != "reduced" {
			full = porcupine.CheckOperationsTimeout(RegisterModel, toOperations(ops, cut), to)
		}
		line := fmt.Sprintf("key %d ops %d full=%s (%.0fs)", k, len(ops), full, time.Since(t0).Seconds())
		if full == porcupine.Unknown {
			kept, dropped := reduceKey(ops)
			t1 := time.Now()
			red := porcupine.CheckOperationsTimeout(RegisterModel, toOperations(kept, math.MaxInt64), to)
			line += fmt.Sprintf(" reduced(dropped %d)=%s (%.0fs)", dropped, red, time.Since(t1).Seconds())
		}
		fmt.Println(line)
	}
}
