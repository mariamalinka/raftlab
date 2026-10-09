package harness

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// Op is one completed (or abandoned) client operation in the history.
type Op struct {
	Process  int    `json:"process"`
	F        string `json:"f"` // read, write, cas
	Key      int    `json:"key"`
	Value    int    `json:"value"`    // written value, CAS new value, or value read (-1 = missing)
	Expected int    `json:"expected"` // CAS only
	Type     string `json:"type"`     // ok, fail, info (indeterminate)
	CASOK    bool   `json:"cas_ok"`   // CAS only, when Type == ok
	Call     int64  `json:"call"`     // ns since test start
	Return   int64  `json:"return"`
	Err      string `json:"err,omitempty"`
}

type WorkloadOptions struct {
	Clients int
	// KeyRotate moves the workload to a fresh set of keys this often, so each
	// key's history stays short enough for the checker (Jepsen "independent").
	KeyRotate  time.Duration
	ActiveKeys int
	OpTimeout  time.Duration
	// Stagger is the mean pause between one client's operations.
	Stagger time.Duration
	Values  int // writes pick values in [0, Values)
	// UniqueValues gives every write a value no other write uses, so a stale
	// read cannot be explained by an indeterminate write of the same value.
	// CAS then expects the last value this client saw on the key.
	UniqueValues bool
}

func (o *WorkloadOptions) defaults() {
	if o.Clients == 0 {
		o.Clients = 10
	}
	if o.KeyRotate == 0 {
		o.KeyRotate = 5 * time.Second
	}
	if o.ActiveKeys == 0 {
		o.ActiveKeys = 2
	}
	if o.OpTimeout == 0 {
		o.OpTimeout = time.Second
	}
	if o.Stagger == 0 {
		o.Stagger = 20 * time.Millisecond
	}
	if o.Values == 0 {
		o.Values = 5
	}
}

// RunRegister drives the read/write/CAS register workload until ctx ends and
// returns the history.
func RunRegister(ctx context.Context, db DB, start time.Time, o WorkloadOptions) ([]Op, error) {
	o.defaults()
	var (
		mu      sync.Mutex
		history []Op
		wg      sync.WaitGroup
		errOnce error
	)
	since := func() int64 { return int64(time.Since(start)) }
	for p := range o.Clients {
		cl, err := db.NewClient()
		if err != nil {
			return nil, err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cl.Close()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(p)))
			process := p
			next := 0
			seen := map[int]int{} // key -> last value this client read or wrote
			value := func() int {
				if !o.UniqueValues {
					return rng.Intn(o.Values)
				}
				next++
				return next*o.Clients + p
			}
			for ctx.Err() == nil {
				epoch := int(time.Since(start) / o.KeyRotate)
				key := epoch*o.ActiveKeys + rng.Intn(o.ActiveKeys)
				ks := "r" + strconv.Itoa(key)
				op := Op{Process: process, Key: key}
				octx, cancel := context.WithTimeout(context.Background(), o.OpTimeout)
				op.Call = since()
				f := rng.Intn(3)
				if f == 2 && o.UniqueValues {
					if _, ok := seen[key]; !ok {
						f = 1 // nothing to compare against yet
					}
				}
				switch f {
				case 0:
					op.F = "read"
					v, found, err := cl.Get(octx, ks)
					op.Value = -1
					if err != nil {
						op.Type, op.Err = "fail", err.Error()
					} else {
						op.Type = "ok"
						if found {
							op.Value, err = strconv.Atoi(v)
							if err != nil {
								op.Type, op.Err = "fail", "bad value "+v
							} else {
								seen[key] = op.Value
							}
						}
					}
				case 1:
					op.F = "write"
					op.Value = value()
					op.Type, op.Err = classify(cl.Put(octx, ks, strconv.Itoa(op.Value)))
					if op.Type == "ok" {
						seen[key] = op.Value
					}
				case 2:
					op.F = "cas"
					if o.UniqueValues {
						op.Expected, op.Value = seen[key], value()
					} else {
						op.Expected, op.Value = rng.Intn(o.Values), rng.Intn(o.Values)
					}
					ok, err := cl.CAS(octx, ks, strconv.Itoa(op.Expected), strconv.Itoa(op.Value))
					op.Type, op.Err = classify(err)
					op.CASOK = ok
					if op.Type == "ok" && ok {
						seen[key] = op.Value
					}
				}
				op.Return = since()
				cancel()
				mu.Lock()
				history = append(history, op)
				mu.Unlock()
				if op.Type == "info" {
					// Like Jepsen, a process with an indeterminate op is
					// retired; a fresh process id takes its place.
					process += o.Clients
				}
				time.Sleep(time.Duration(rng.Int63n(int64(2*o.Stagger) + 1)))
			}
		}()
	}
	wg.Wait()
	return history, errOnce
}

func classify(err error) (string, string) {
	switch {
	case err == nil:
		return "ok", ""
	case errors.Is(err, ErrIndeterminate):
		return "info", err.Error()
	default:
		return "fail", err.Error()
	}
}
