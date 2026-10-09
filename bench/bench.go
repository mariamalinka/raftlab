// Package bench is the open-loop load generator used for E3, E5 and E7.
//
// Requests are issued on a fixed schedule (arrival rate R) regardless of how
// fast earlier ones complete, and latency is measured from each request's
// intended start time. A closed-loop client would wait for slow responses
// before sending more, hiding queueing delay (coordinated omission).
package bench

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// Backend is one system under test. Do may be called concurrently.
type Backend interface {
	Name() string
	Get(ctx context.Context, key string) error
	Put(ctx context.Context, key, value string) error
	Close()
}

type Params struct {
	Rate      float64 // offered load, ops/s
	Duration  time.Duration
	Warmup    time.Duration // excluded from the histogram
	ReadFrac  float64
	ValueSize int
	Keys      int
	Zipf      bool
	Timeout   time.Duration
	// MaxOutstanding caps concurrent requests; arrivals beyond it count as
	// errors (the system is past saturation).
	MaxOutstanding int
}

type Result struct {
	Backend     string  `json:"backend"`
	Rate        float64 `json:"offered_ops_s"`
	Throughput  float64 `json:"achieved_ops_s"`
	ReadFrac    float64 `json:"read_frac"`
	ValueSize   int     `json:"value_size"`
	Zipf        bool    `json:"zipf"`
	Ops         int64   `json:"ops"`
	Errors      int64   `json:"errors"`
	Dropped     int64   `json:"dropped"`
	P50         float64 `json:"p50_ms"`
	P99         float64 `json:"p99_ms"`
	P999        float64 `json:"p999_ms"`
	Max         float64 `json:"max_ms"`
	Mean        float64 `json:"mean_ms"`
	Percentiles string  `json:"-"`
}

func keyName(i int) string { return "k" + strconv.Itoa(i) }

// Preload writes every key once so reads hit real values.
func Preload(ctx context.Context, b Backend, keys, valueSize, parallel int) error {
	val := string(make([]byte, valueSize))
	var wg sync.WaitGroup
	var firstErr atomic.Value
	ch := make(chan int)
	for range parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := range ch {
				for attempt := 0; ; attempt++ {
					c, cancel := context.WithTimeout(ctx, 5*time.Second)
					err := b.Put(c, keyName(k), val)
					cancel()
					if err == nil {
						break
					}
					if attempt > 5 {
						firstErr.CompareAndSwap(nil, err)
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}
	for k := range keys {
		ch <- k
	}
	close(ch)
	wg.Wait()
	if err, _ := firstErr.Load().(error); err != nil {
		return fmt.Errorf("preload: %w", err)
	}
	return nil
}

func Run(ctx context.Context, b Backend, p Params) Result {
	if p.Timeout == 0 {
		p.Timeout = 5 * time.Second
	}
	if p.MaxOutstanding == 0 {
		p.MaxOutstanding = 20000
	}
	hist := hdrhistogram.New(1, int64(60*time.Second/time.Microsecond), 3) // µs
	var hmu sync.Mutex
	var ops, errs, dropped atomic.Int64
	var outstanding atomic.Int64

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var zipf *rand.Zipf
	if p.Zipf {
		zipf = rand.NewZipf(rng, 1.01, 1, uint64(p.Keys-1))
	}
	value := string(make([]byte, p.ValueSize))
	interval := time.Duration(float64(time.Second) / p.Rate)
	start := time.Now()
	measureFrom := start.Add(p.Warmup)
	end := measureFrom.Add(p.Duration)
	var wg sync.WaitGroup
	for k := 0; ; k++ {
		intended := start.Add(time.Duration(k) * interval)
		if !intended.Before(end) || ctx.Err() != nil {
			break
		}
		if d := time.Until(intended); d > 0 {
			time.Sleep(d)
		}
		var key int
		if zipf != nil {
			key = int(zipf.Uint64())
		} else {
			key = rng.Intn(p.Keys)
		}
		read := rng.Float64() < p.ReadFrac
		measured := !intended.Before(measureFrom)
		if outstanding.Load() >= int64(p.MaxOutstanding) {
			if measured {
				dropped.Add(1)
			}
			continue
		}
		outstanding.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer outstanding.Add(-1)
			c, cancel := context.WithTimeout(ctx, p.Timeout)
			var err error
			if read {
				err = b.Get(c, keyName(key))
			} else {
				err = b.Put(c, keyName(key), value)
			}
			cancel()
			lat := time.Since(intended)
			if !measured {
				return
			}
			if err != nil {
				errs.Add(1)
				return
			}
			ops.Add(1)
			hmu.Lock()
			hist.RecordValue(max(1, lat.Microseconds()))
			hmu.Unlock()
		}()
	}
	wg.Wait()
	ms := func(us int64) float64 { return float64(us) / 1000 }
	r := Result{
		Backend: b.Name(), Rate: p.Rate, ReadFrac: p.ReadFrac, ValueSize: p.ValueSize, Zipf: p.Zipf,
		Ops: ops.Load(), Errors: errs.Load(), Dropped: dropped.Load(),
		Throughput: float64(ops.Load()) / p.Duration.Seconds(),
		P50:        ms(hist.ValueAtQuantile(50)),
		P99:        ms(hist.ValueAtQuantile(99)),
		P999:       ms(hist.ValueAtQuantile(99.9)),
		Max:        ms(hist.Max()),
		Mean:       hist.Mean() / 1000,
	}
	var buf []byte
	for _, bk := range hist.CumulativeDistribution() {
		buf = fmt.Appendf(buf, "%.4f\t%.3f\t%d\n", bk.Quantile, ms(bk.ValueAt), bk.Count)
	}
	r.Percentiles = string(buf)
	return r
}
