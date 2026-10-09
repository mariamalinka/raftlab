// Package clock provides the time source every Raft timer reads.
//
// Real deployments use the wall clock. The fault harness needs to inject clock
// skew (hypothesis H1), and libfaketime is not available everywhere, so the
// node routes all timing through a Skewable clock whose offset and rate can be
// changed at runtime over the admin API. Timers are evaluated by comparing
// Now() against deadlines in a fast ticker loop, so skew takes effect at once.
package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

// Skewable is a clock with an adjustable offset (jumps) and rate (drift).
//
// Now() = anchorClock + (realNow - anchorReal) * rate.
// Changing the rate re-anchors so the clock stays continuous.
type Skewable struct {
	mu          sync.Mutex
	anchorReal  time.Time
	anchorClock time.Time
	rate        float64
}

func NewSkewable() *Skewable {
	now := time.Now()
	return &Skewable{anchorReal: now, anchorClock: now, rate: 1}
}

func (c *Skewable) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nowLocked(time.Now())
}

func (c *Skewable) nowLocked(real time.Time) time.Time {
	elapsed := real.Sub(c.anchorReal)
	return c.anchorClock.Add(time.Duration(float64(elapsed) * c.rate))
}

// Jump moves the clock by d (negative jumps go backwards).
func (c *Skewable) Jump(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	real := time.Now()
	c.anchorClock = c.nowLocked(real).Add(d)
	c.anchorReal = real
}

// SetRate makes the clock run rate times as fast as real time.
func (c *Skewable) SetRate(rate float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	real := time.Now()
	c.anchorClock = c.nowLocked(real)
	c.anchorReal = real
	c.rate = rate
}

// Reset returns the clock to real time at normal rate.
func (c *Skewable) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.anchorReal, c.anchorClock, c.rate = now, now, 1
}

// Skew reports the current offset from real time and the rate.
func (c *Skewable) Skew() (time.Duration, float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	real := time.Now()
	return c.nowLocked(real).Sub(real), c.rate
}
