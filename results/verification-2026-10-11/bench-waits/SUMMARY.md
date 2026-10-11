# AppendEntries durability waits on followers (instrumented cfa86fa, scratch build, not in the repo)

One loadgen invocation per point; counters cover preload + warmup + 20 s measured, summed over the 4 followers.
'with entries' = requests that appended new entries (these waited before the fix too); 'no new entries' = heartbeats, ReadIndex probes, retransmissions (waits added by the fix).

| read frac | ops/s | with entries: acks / waited / mean wait | no new entries: acks / waited (%) / mean wait | no new entries: waits >= 6.4 ms |
|---|---|---|---|---|
| 0 | 1000 | 82398 / 82398 / 1.05 ms | 3959 / 3392 (86%) / 1.60 ms | 136 (4.0%) |
| 0 | 4000 | 60154 / 60154 / 1.64 ms | 13706 / 12553 (92%) / 1.74 ms | 419 (3.3%) |
| 0 | 8000 | 34583 / 34583 / 2.73 ms | 9873 / 8514 (86%) / 3.03 ms | 889 (10.4%) |
| 0.5 | 1000 | 46338 / 46338 / 0.98 ms | 46455 / 11609 (25%) / 1.03 ms | 261 (2.2%) |
| 0.5 | 4000 | 60920 / 60920 / 1.41 ms | 92204 / 61820 (67%) / 1.40 ms | 1149 (1.9%) |
| 0.5 | 8000 | 39768 / 39768 / 1.93 ms | 72758 / 41770 (57%) / 2.05 ms | 2177 (5.2%) |
Histogram bucket i counts waits < 50us*2^i (raw lines in per-rate/*/instr-n*.txt). The first attempt (bench-waits/r0.5, r0) lost its node logs at teardown; only its stdout is kept.
