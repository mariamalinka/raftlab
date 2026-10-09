package bench

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"raftkv/harness"
)

type FailoverResult struct {
	Trial  int `json:"trial"`
	Leader int `json:"leader"`
	// GapMS is the time from the leader kill to the first successful write
	// that was issued after the kill.
	GapMS float64 `json:"gap_ms"`
	// CatchUpMS is how long the restarted node took to apply everything the
	// leader had committed when it restarted.
	CatchUpMS float64 `json:"catch_up_ms"`
}

// Failover kills the leader under a steady write load, measures the
// unavailability window, restarts the node and measures its catch-up.
func Failover(ctx context.Context, db harness.ClusterDB, b Backend, trials int, rate float64, valueSize, keys int) []FailoverResult {
	var out []FailoverResult
	value := string(make([]byte, valueSize))
	for t := 1; t <= trials; t++ {
		res := FailoverResult{Trial: t}
		lctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		leader, err := db.Leader(lctx)
		cancel()
		if err != nil {
			continue
		}
		res.Leader = leader

		var mu sync.Mutex
		var killAt time.Time
		var firstOK time.Time
		stop := make(chan struct{})
		var wg sync.WaitGroup
		interval := time.Duration(float64(time.Second) / rate)
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.NewTicker(interval)
			defer tick.Stop()
			for {
				select {
				case <-stop:
					return
				case <-tick.C:
				}
				issued := time.Now()
				wg.Add(1)
				go func() {
					defer wg.Done()
					c, cancel := context.WithTimeout(ctx, 10*time.Second)
					err := b.Put(c, keyName(rand.Intn(keys)), value)
					cancel()
					done := time.Now()
					mu.Lock()
					if err == nil && !killAt.IsZero() && !issued.Before(killAt) && (firstOK.IsZero() || done.Before(firstOK)) {
						firstOK = done
					}
					mu.Unlock()
				}()
			}
		}()
		time.Sleep(2 * time.Second)
		mu.Lock()
		killAt = time.Now()
		mu.Unlock()
		db.Kill(leader)
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			done := !firstOK.IsZero()
			mu.Unlock()
			if done {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		mu.Lock()
		if !firstOK.IsZero() {
			res.GapMS = float64(firstOK.Sub(killAt).Microseconds()) / 1000
		} else {
			res.GapMS = -1
		}
		mu.Unlock()

		// Restart the old leader while load continues; time its catch-up.
		time.Sleep(time.Second)
		restart := time.Now()
		db.Start(leader)
		nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		newLeader, _ := db.Leader(nctx)
		cancel()
		var target uint64
		if newLeader >= 0 {
			c, cancel := context.WithTimeout(ctx, time.Second)
			target, _ = db.Applied(c, newLeader)
			cancel()
		}
		res.CatchUpMS = -1
		for time.Since(restart) < 60*time.Second {
			c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			applied, err := db.Applied(c, leader)
			cancel()
			if err == nil && applied >= target {
				res.CatchUpMS = float64(time.Since(restart).Microseconds()) / 1000
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		close(stop)
		wg.Wait()
		out = append(out, res)
		time.Sleep(2 * time.Second)
	}
	return out
}
