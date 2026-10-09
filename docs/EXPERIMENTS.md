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
  ReadIndex was linearizable in 3/3. First evidence for H1.
- E6: only localread showed signs (2/3 "unknown", i.e. checker timeouts; an
  earlier scratch run caught it in 2/3). The other six bugs went undetected
  on their expected fault in 3/3 trials. Next: confirm each bug is
  reachable with a targeted unit test, raise the checker timeout, and lengthen
  or intensify the fault schedules.
