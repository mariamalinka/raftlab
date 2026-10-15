# Limited E6: can the harness detect ackbeforefsync and localread? (2026-10-15)

## 1. What was tested

- **Code:** GitHub `main` at `85a3e22`. It contains the deterministic bug tests (`a2ffbb4`), the E6 script fixes (`30e86b0`) and the combined detection column (`85a3e22`). Nothing was reapplied or changed. Binaries were built from that commit, and `make bugtests` gave 7/7 caught.
- **Settings:** 5 nodes and 25 clients. Every flag is the `E6H` line of `scripts/experiment.sh`:
  - `-time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s`
  - `-key-rotate 12s -unique-values`
  - the readindex read path
- **Cells:** each bug runs under its target fault from the plan's bug table. Each control is the bug-free build with exactly the same flags.

| Cell | Bug | Fault | Extra flag |
|---|---|---|---|
| ackbeforefsync/kill-all | ackbeforefsync | kill-all (crash every node) | `-lazyfs` |
| correct-lazyfs/kill-all | none | kill-all | `-lazyfs` |
| localread/partition-leader | localread | partition-leader | none |
| correct/partition-leader | none | partition-leader | none |

There were 20 attempts per cell, 80 in total, from 00:19 to 02:08 UTC. The other five bugs and the full matrix were not run.

## 2. Results

| | ackbeforefsync | control (LazyFS) | localread | control |
|---|---|---|---|---|
| Attempts | 20 | 20 | 20 | 20 |
| Completed (checker finished) | 19 | 20 | 16 | 20 |
| Checker crash (out of memory) | **1** | 0 | **4** | 0 |
| Ok / Ok (reduced) | 0 / 0 | 18 / 2 | 1 / 1 | 16 / 4 |
| **Unknown** (kept separate) | **19** | 0 | **13** | 0 |
| Porcupine Illegal | 0 | 0 | 1 | 0 |
| Trials with provably stale reads | 19/19 (160 reads) | 0/20 | 14/16 (48 reads) | 0/20 |
| **Combined detection** [95% CI] | **19/19 = 1.00 [0.83, 1.00]** | 0/20 = 0.00 [0.00, 0.16] | **14/16 = 0.88 [0.64, 0.97]** | 0/20 = 0.00 [0.00, 0.16] |
| Porcupine-only detection [95% CI] | 0/0, undefined [0.00, 1.00] | 0/20 = 0.00 [0.00, 0.16] | 1/3 = 0.33 [0.06, 0.79] | 0/20 = 0.00 [0.00, 0.16] |
| Median detection time, Porcupine | - | - | 4.2 s (n = 1) | - |
| Median time of the first stale read | 9.1 s | - | 4.7 s | - |
| Faults planned / executed / completed | 3/3/3 in every trial | 3/3/3 | 3/3/3 | 3/3/3 |
| Shortest fault | 5.00 s | 5.00 s | 5.08 s | 5.07 s |
| Bug flag on every node start | yes, 380/380 | n/a | yes, 80/80 | n/a |

How to read the table:

- **Combined detection** counts a trial as detected when it is Illegal or has at least one provably stale read. The denominator is every completed trial, Unknown included.
- **Porcupine-only detection** is Illegal over Ok + Ok (reduced) + Illegal. Unknown is excluded from that denominator.
- The faults-per-trial row covers completed trials. The crashed trials' fault logs also show 3 faults of at least 5 s each.

**Bug activation.**
- Every node start in every completed bug trial logged the requested bug.
- The stale reads are the bugs' own effects:
  - ackbeforefsync: acknowledged writes lost after a crash, then reads returning older values.
  - localread: the isolated old leader answering reads.
- The bug-free controls, with the same faults and flags, never produced one.

**Checker crashes.** Five rkharness processes were killed while checking a trial.
- Four kills are confirmed by the kernel's out-of-memory log, at 13.8 to 13.9 GB on a 16 GB machine (`analysis/oom-kills.txt`).
- The fifth, the first ackbeforefsync process, was killed by the same signal but left no kernel log line. Out of memory is presumed but not confirmed.
- The 25 resumed trials that finished peaked at a median of 9.2 GB and a maximum of 13.0 GB (`raw/E6/attempts.log`).
- After the first two kills, the remaining trials ran one per process with identical flags, so one crash cost only one trial.
- The crashed trials are counted as crashes, not as Ok, Unknown or Illegal.
- Their histories are saved. An offline scan of those histories (same rule as the harness) finds stale reads in all 5: 3 for ackbeforefsync, and 7, 14, 2 and 8 for localread. These are reported here only and are not in any rate.
- Node logs for the 5 crashed trials were not written, because the harness saves them only after checking.

## 3. Does the harness detect these two bugs?

**Yes, through the stale-read scan.**
- ackbeforefsync was detected in 19 of 19 completed trials. The lower bound of the 95% CI is 0.83.
- localread was detected in 14 of 16, with a lower bound of 0.64.

**No, through Porcupine alone.** It reached a verdict in none of the ackbeforefsync trials and in only 3 of the 16 localread trials (1 Illegal). With unique values and histories that contain violations, Porcupine almost always times out. So Porcupine-only detection here measures the checker's limits more than the harness's ability to find bugs.

## 4. Do the bug-free controls pass?

**Yes.** Both controls had 40 of 40 trials linearizable (34 Ok and 6 Ok (reduced)), with 0 Unknown, 0 Illegal and 0 stale reads. This puts an upper bound of about 0.16 (95%) on the false-positive rate of either detector under these settings.

## 5. Limitations

- **Unknown dominates the bug cells.** Porcupine is not a usable primary detector for histories with violations under this workload. The result rests on the stale-read scan, which is sound (it flags only reads no linearization can explain) but sees only stale reads. A bug that causes other anomalies would need Porcupine.
- **Checker memory.** The checker needs up to 13 GB per trial on these histories (median 9.2 GB), and 5 of 40 bug trials crashed. A full E6 needs either more memory or one trial per process, so a crash costs only one trial.
- **One fault per bug.** These are the bugs' target faults. H2, which asks which faults expose which bugs, is not tested.
- **The injected bug is known to be on, not known to fire on any given operation.** There are no hit counters. Here the stale reads show the bug's effect directly, so this matters less for these two bugs than for the others.
- **Small samples.** With 16 to 20 trials per cell, the CIs are wide. One environment: a single 4-vCPU container, real network namespaces, LazyFS simulated in the WAL.

## 6. Enough for a smaller academic project?

Yes, as a focused claim. Under its target fault, each of two known Raft bugs was detected with a probability of at least 0.83 (ackbeforefsync) and 0.64 (localread), as 95% lower bounds, while the correct build had zero false positives in 40 trials. That is a quantitative RQ3 result of the kind the plan asks for, together with G1 (1,500/1,500 runs) and the deterministic tests for all 7 bugs. It does not support claims about H2, or about the other five bugs.

## 7. What else is needed

**Essential:**
- E1 under the same settings, the correct build across all faults. These controls only cover two faults.
- The E2 etcd control, to show the harness passes a production system.
- Reporting the checker crash rate and the Unknown rate as results in their own right, with the same caveats as above.

**Strongly recommended, cheap:** the same two bugs under one non-target fault each. For example, ackbeforefsync under partition-leader, where it should be undetectable, and localread under pause-leader. This gives a first look at H2. It would be 4 more cells.

**Reasonable future work:**
- The remaining five bugs. figure8, notruncate and votenopersist are expected to be near 0, and the deterministic tests cover them.
- The full 74-cell matrix.
- Bug-hit counters (these need Raft-core changes).
- Reducing checker memory.
- A dedicated votenopersist fault.

## Reproduce

On Linux, as root, with `iproute2` and Go 1.26, at commit `85a3e22`:

```
export BIN=/tmp/raftkv-bin-e6l && make build && make bugtests
bash analysis/run.sh        # 4 cells x 20 trials (copy of the script used)
bash analysis/resume.sh     # same flags, one trial per process
python3 analysis/aggregate.py raw > analysis/RESULTS.md
```

## Files

Everything is under `research/e6-limited-2026-10-15/`. Every earlier artifact is untouched.

- `raw/E6/<cell>/trial-NNN/` and `raw/E6/<cell>-single/attempt-NNN/trial-001/`: history, nemesis events, result, node logs and command line for every trial.
- `raw/E6/attempts.log`: exit code and peak memory per resumed trial.
- `analysis/RESULTS.md`: the table above plus every trial.
- `analysis/aggregate.py`, `run.sh`, `resume.sh`, `run.log`, `oom-kills.txt`.
