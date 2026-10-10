# Experiment log

The plan's matrix, how each experiment runs here, and where it stands.
Full runs use `make eN` (20 trials per cell, 30 s each, 25 clients; benchmarks
5 reps per point). Pilot runs use 3 trials of 20 s.

| ID | Question | Command | Status |
|---|---|---|---|
| G1 | Raft suite passes 500 runs | `make g1` | Suite passes in all three modes (default, pipelined, unbatched) on single runs. 500-run gate not yet run: ~150 s per run, so ~21 h per mode on a 4-core machine. |
| E1 | RQ1: raftkv linearizable under each nemesis | `make e1` | Pilot done (see results/pilot-2026-10-09). |
| E2 | Harness validity: etcd passes | `make e2` | Pilot done. |
| E3 | RQ2: latency vs load, both systems | `make e3` | Load generator smoke-tested on both systems; full sweep not run. |
| E4 | RQ2: failover gap and catch-up | `make e4` | Smoke-tested (2 kills each); full run not done. |
| E5 | H1: read paths, speed vs correctness | `make e5` | Correctness pilot under `clock-partition` done; speed side not run. |
| E6 | RQ3/H2: detection rate per bug × fault | `make e6` | Pilot of each bug on its expected fault done. Full matrix: 7 bugs × 8 faults × 20 trials ≈ 1,100 trials ≈ 13 h. |
| E7 | H3: ablation vs etcd | `make e7` | Not run. |
| E8 | etcd unsafe serializable reads | `make e8` | Not run. |

## Fault catalogue (`harness/nemesis.go`)

| Name | What happens |
|---|---|
| kill-random, kill-leader, kill-majority, kill-all | `kill -9`, restart on heal |
| pause-random, pause-leader | SIGSTOP, SIGCONT on heal |
| partition-leader | leader isolated from everyone |
| partition-halves | random 2 / 3 split |
| partition-bridge | two halves that can't talk, one node sees both |
| clock-skew | random nodes jump ±2 election timeouts and run 0.25×–4× (raftkv only) |
| clock-partition | leader's clock at 0.25× speed, then isolated (beyond any drift bound) |
| clock-partition-mild | same at 0.92× (inside the default 10% bound) |
| bridge-churn | partition-bridge with the bridge node killed and restarted every 100-400 ms (targets votenopersist) |
| mixed | each round picks one of the above process, network or clock faults (H2) |

`-lazyfs` makes raftkv lose unsynced WAL writes on kill.

## Bug → fault pairing used in the pilot

| Bug flag | Plan's description | Fault | Notes |
|---|---|---|---|
| figure8 | Commit previous-term entries by counting replicas | mixed | |
| localread | Serve reads on the leader without a quorum check | partition-leader | |
| leasenodrift | Lease read without a clock-drift bound | clock-partition-mild, read path lease | |
| votenopersist | Grant a vote before persisting votedFor | kill-leader | |
| notruncate | Skip truncating a conflicting log suffix | partition-halves | |
| nodedup | Apply a retried request twice | partition-leader | |
| ackbeforefsync | Acknowledge before fsync | kill-all with -lazyfs | fsyncs only every 500 ms in the background |

## Pilot results, 2026-10-09 (3 trials × 20 s, 25 clients)

Full table: results/pilot-2026-10-09/REPORT.md.

- E2: etcd linearizable in 12/12 trials (4 faults). No harness false positives.
- E1: raftkv linearizable in every decided trial across 10 fault types. One
  partition-bridge trial was "unknown" (checker timeout).
- E5: under clock-partition (leader clock at 0.25×, then isolated), lease
  reads were non-linearizable in 3/3 trials (median detection 5.2 s);
  ReadIndex was linearizable in 3/3. The 0.25x clock is far outside the 10%
  drift bound the lease assumes, so this shows what happens when that
  assumption breaks, not a lease defect: every provably stale read came from
  clients pinned to the slowed leader, inside the predicted lease window.
- E6: 0 of 7 bugs detected (0 anomalies in 21 trials). localread gave 2
  "unknown" results, which are checker timeouts, not detections. An earlier
  30 s run of localread (archived in results/scratch-2026-10-09) caught it in
  2/3. An earlier version of this log said "6 of 7 missed"; that over-counted
  the timeouts. Why each bug was missed, with evidence:
  ../E5-E6-INVESTIGATION.md (in the research folder). Main causes: keys
  rotate every 5 s, as long as each fault, so post-recovery damage is never
  read; a 5-value domain lets indeterminate writes explain stale reads; the
  third fault of a 20 s trial is cut below one election timeout.

## Rerun after the E6 investigation, 2026-10-09 (3 trials × 20 s, 25 clients)

Full table and stale-read scan: results/rerun-2026-10-09/REPORT.md. New
harness options: `-key-rotate`, `-active-keys`, `-values`, `-unique-values`.

| Run | Porcupine | Trials with provably stale reads |
|---|---|---|
| ackbeforefsync, kill-all, lazyfs, key-rotate 15 s | Ok 3/3 | 0/3 |
| ackbeforefsync, same + unique values | Unknown 3/3 (timeout) | 3/3, all just after a restart |
| correct build, same + unique values (control) | Ok 3/3 | 0/3 |
| localread, partition-leader, unique values | Unknown 3/3 (timeout) | 2/3, all from clients of the isolated leader |
| localread, clock-partition, unique values | Illegal 3/3 | 3/3 |
| correct build, clock-partition, unique values (control) | Ok 3/3 | 0/3 |

- Longer key rotation alone did not expose ackbeforefsync: with 5 values,
  writes left indeterminate by the kill explain the stale reads. Unique values
  are what make the loss visible.
- With unique values and long-lived keys, Porcupine times out. Until the
  checker scales, report the stale-read scan next to its verdict
  (`analysis/stale_reads.py`).

## Deterministic bug tests (RQ3 reachability)

`make bugtests` runs one deterministic test per bug with that bug switched on
and checks that it fails: `TestBugVoteNoPersist`, `TestBugNoTruncate`,
`TestBugFigure8` (internal/raft/bugs_test.go) and `TestBugNoDedup`
(internal/kv/bugs_test.go). All four pass in normal builds and fail 20/20
with their bug on, under the race detector. ackbeforefsync, localread and
leasenodrift are not covered yet.

## Fault schedule changes, 2026-10-10

- A fault that cannot run its full `-fault-duration` before the end of the
  run is no longer started (it used to be cut to under a second).
- New fault `bridge-churn` (see catalogue). New flag `-raftkv-args` passes node
  flags such as `-batch=8` through to every raftkv node.
- Validation (3 trials each, unique values, key-rotate 15 s; results/faults-2026-10-10):
  votenopersist/bridge-churn, leasenodrift/clock-partition-mild at T = 3 s and
  notruncate/partition-halves with -batch=8 found nothing (Ok or checker timeout,
  no provably stale reads). **The bug-free build lost an acknowledged write
  under bridge-churn in 1 of 3 trials**; see that REPORT.md. Likely cause:
  deep-dive finding 16.1 (follower acks entries before they are persisted).
