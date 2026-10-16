# RQ2 and recovery: E3, overload diagnostics, limited E7, E4

Date: 2026-10-16, 17:29–18:45 UTC (2 h budget). Commit: `14bf0f6` (your device's `main` and the clean clone used here; working tree clean). R-1 durability fix present (`00d9651` "raft: follower acknowledges AppendEntries only after its log is stored", plus `internal/raft/durability_test.go`). No Raft core, checker or configuration changes. Earlier result folders were not modified.
Figures: https://claude.ai/artifact/6oq6W7nmCF6tcNioTdZZZ6 (source in `figures/`).

## What was completed, reduced or skipped

| Item | Status | Scope actually run |
|---|---|---|
| E3 RaftKV vs etcd | **Completed, reduced reps** | 3 mixes × 1k/2k/4k/8k ops/s. 95/5: etcd 5 reps; RaftKV 7 reps (5 reused from the E5 extension, same commit and method, + 2 new). 50/50 and 100% writes: **4 reps** per system (not 5). |
| Overload diagnostics | **Completed (small)** | 50/50 at 8k–14k: RaftKV 2 reps, etcd 1 rep; RaftKV write-only 6k–10k, 1 rep; 1 goroutine-dump run at 12k. |
| E7 ablation | **Completed, reduced** | 50/50 at 2k and 8k; default, batching off, pipelining off, fsync off (UNSAFE); **3 reps** each (not 5). |
| E4 kill-leader | **Completed** | 10 leader kills per system (new, loadgen failover mode) + 60 fault events per system re-analysed from E1/E2. |
| E4 partition-leader | **Completed from existing data, no new runs** | 60 fault events per system re-analysed from the E1/E2 histories of 2026-10-15 (20 trials × 3 faults). loadgen's failover mode supports only kill-leader. |
| Goroutine and queue metrics | **Partial** | RaftKV exposes no metrics endpoint. Queue growth was measured as process memory; goroutines were counted from one SIGQUIT dump. |
| fsync counts | **Not measurable** | Only disk bytes written and write syscalls per second (`/proc/<pid>/io`). |
| etcd election time | **Not observable** | etcd runs at `--log-level warn`, so elections are not logged. |

Settings for all bench runs: 5 nodes, election timeout 1 s, heartbeat 100 ms, open-loop loadgen, 10,000 keys, 128 B values, 5 s warmup, 20 s window, fresh cluster per repetition, 5 s client timeout, client in-flight cap 20,000. RaftKV uses ReadIndex reads, fsync on, batch 256, pipelining on; etcd 3.7.2 uses linearizable reads and fsync on. Both run in the same 4 vCPU / 16 GB container with the client. The harness starts RaftKV with `-snapshot-bytes 262144` (snapshot every 256 KB of WAL).

## 1. E3 performance (medians; [min–max] across reps)

Achieved throughput equalled offered load at every point for both systems, with 0 errors and 0 drops, except one RaftKV run (below).

| Mix | Load | RaftKV p50 / p99 (ms) | etcd p50 / p99 (ms) |
|---|---|---|---|
| 95/5 | 1,000 | 1.13 / 3.8 | 1.19 / 4.1 |
| 95/5 | 2,000 | 1.10 / 7.4 | 1.25 / 6.5 |
| 95/5 | 4,000 | 2.20 / 11.1 | 2.72 / 15.8 |
| 95/5 | 8,000 | 4.74 / 22.6 | 7.62 / 28.7 |
| 50/50 | 1,000 | 1.48 / 7.2 | 1.82 / 6.5 |
| 50/50 | 2,000 | 2.10 / 17.6 | 3.30 / 12.8 |
| 50/50 | 4,000 | 4.00 / 38.9 | 7.48 / 25.7 |
| 50/50 | 8,000 | 8.62 / 48.5 | 16.9 / 47.6 |
| 100% writes | 1,000 | 1.80 / 11.9 | 2.41 / 8.3 |
| 100% writes | 2,000 | 2.83 / 31.2 | 4.40 / 25.3 |
| 100% writes | 4,000 | 4.27 / 40.6 | 8.58 / 74.5 |
| 100% writes | 8,000 | 10.9 / 66.1 (one rep collapsed) | 24.6 / 118.5 |

- **p50:** RaftKV is lower than etcd at all 12 points, by 1.1–1.6× (95/5) and 1.2–2.3× (50/50 and write-only).
- **p99:** mixed. etcd is lower in the 50/50 mix at 1k–4k and write-only at 1k–2k; RaftKV is lower at 4k–8k write-only and in 95/5 at 4k–8k. p99 ranges between reps often overlap, so p99 orderings are preliminary.
- **Instability:** in 1 of 4 RaftKV write-only reps (rep 3), the 8,000 ops/s point collapsed: 3,949 ops/s achieved, 40,387 errors, 40,624 dropped, p99 1.09 s, p99.9 5.07 s. It is included in all statistics (Section 2).
- **Resources:** both systems keep the host CPU-saturated (node CPU sum about 240–260% of 400%, CPU pressure 50–75%). RaftKV's busiest node (the leader) uses 100–125% CPU against etcd's 65–75%. RaftKV writes 2–3× more bytes to disk at the same load (write-only 8k: 62 vs 23 MB/s). Node memory: RaftKV ≤ 43 MB, etcd ≤ 66 MB (excluding the collapse).
- Data: `analysis/bench.md`, `analysis/bench-per-rep.csv`, `analysis/bench-summary.json`.

## 2. Overload investigation

**Results.**
- 50/50, 8k–14k ops/s: RaftKV served the full load in both reps with 0 errors (p99 at 14k: 186 and 301 ms; etcd 168 ms). The collapse at 12,000 ops/s reported on 2026-10-11 did **not** recur.
- Write-only, 6k/8k/10k: RaftKV served the full load with 0 errors.
- The only collapse in this session is the E3 write-only run above, so the collapse is **intermittent** near 8k write-only on this host (1 of 5 RaftKV write-only runs that reached 8k).
- In E7, RaftKV with batching off collapsed at 8k in 3 of 3 reps in exactly the same way (Section 3).

**Verified (from code or measurement):**
1. **No backpressure or admission control.** `Raft.Propose` (`internal/raft/raft.go:959`) appends every proposal to the log and an unbounded persist queue; `Store.propose` (`internal/kv/store.go:309`) returns `ErrTimeout` on client timeout but leaves the proposal in the log, so timed-out work is still replicated, fsynced and applied. The gRPC server starts one goroutine per request with no limit.
2. **Queue growth during the collapse** (E3 write-only rep 3, 1 s samples): leader memory grew from about 30 MB to 366 MB, follower memory to 100–260 MB, and the client from 27 MB to 575 MB (its in-flight requests). No leader change occurred.
3. **Not a storage-pressure bottleneck in the collapse:** IO pressure fell (21% → 10%) while it happened. The leader's write syscalls fell from about 14,000/s to 2,500/s while its bytes written rose from 12 to 23 MB/s.
4. **Not a client limit:** etcd sustained 14,000 ops/s with the same client, and the client used under 1.3 CPU at that load.
5. **Capacity loss turns into collapse.** With batching off (fewer ops per fsync), RaftKV collapsed at 8k in 3/3 reps: 89–412 ops/s achieved, p50 3.7–4.8 s (close to the 5 s timeout), about 150k failed requests per run.
6. **Goroutines (one dump, leader at 12k, not collapsed):** 203 goroutines: 69 writes waiting for apply, 49 reads waiting for ReadIndex confirmation, 32 AppendEntries in flight. Nothing piled up at that point.

**Hypotheses (not verified):**
- The trigger of the intermittent stall. Two followers dropped to about 1% CPU during the collapse, which suggests replication to them stalled. A likely candidate is the very frequent snapshotting (every 256 KB of WAL), which fits the larger writes made with fewer syscalls. Snapshot creation is not logged, so this could not be confirmed.
- CPU saturation as an amplifier: the host is near full CPU at 8k in every configuration, and RaftKV's leader is the busiest process. `waitFor` wakes every waiter on every state change (`raft.go:268`), which costs CPU in proportion to the number of waiters.
- Raft replication overhead itself is unlikely to be the cause, because the same cluster served 14k ops/s in the 50/50 mix.

**Impact on RQ2:** below about 8k ops/s RaftKV is stable and its comparison with etcd is valid. At and above that load, RaftKV can collapse instead of degrading gracefully, while etcd does not. Throughput claims for RaftKV should therefore be limited to 8k ops/s on this host, and the lack of backpressure should be reported as a robustness weakness. A fix (bounded proposal queue or admission control) needs code changes and was not attempted.

## 3. E7 ablation (50/50 mix, 3 reps, medians)

| Configuration | Safety | 2k p50 / p99 (ms) | 8k p50 / p99 (ms) | 8k achieved | Disk write MB/s at 8k |
|---|---|---|---|---|---|
| default (batch 256, pipelining, fsync) | safe | 2.17 / 12.6 | 9.66 / 49.3 | 8,000 | 35.0 |
| batching off (`-batch=1`) | safe | 5.50 / 63.8 | **collapsed**: 3,879 / 5,022 | 253 [89–412] | 47.8 |
| pipelining off (`-pipeline=false`) | safe | 2.03 / 29.4 | 5.16 / 47.6 | 8,000 | 35.0 |
| fsync off (`-fsync=false`) | **UNSAFE** | 0.99 / 12.9 | 4.47 / 31.7 | 8,000 | 24.0 |

- **Batching** is the most important optimization: without it, p50 is 2.5× and p99 5× higher at 2k, and the cluster collapses at 8k.
- **Pipelining** gives no measurable benefit on this single host. Turning it off lowered p50 at 8k (5.2 vs 9.7 ms, ranges do not overlap) and raised p99 at 2k. On one machine with sub-millisecond network latency, pipelining has little latency to hide; this may differ on a real network.
- **fsync off** halves p50 at both loads. It weakens durability (acknowledged writes can be lost in a crash), so it is **not comparable** with the safe configurations and is not a valid optimization. It only bounds the cost of fsync.
- **Does it explain the etcd difference?** RaftKV is not slower than etcd at p50, so there is no p50 gap to explain. The p99 advantage of etcd in the 50/50 mix at 2k–4k is not explained by these flags: the safe flags do not lower RaftKV's p99 below etcd's (12.8 ms at 2k). RaftKV's higher disk write volume (snapshot frequency) and busier leader are the measured differences.

## 4. E4 recovery

Client-visible recovery = time from fault injection to the return of the first write that was **issued after** the fault and **acknowledged by the system** (an OK response, not a client-side retry). Election time = first `became leader` with a higher term after the fault, from RaftKV node logs, mapped to harness time (kill-leader: anchored to the restart log line; partition-leader: first leader time + 37 ms median offset, range 13–62 ms).

| System · fault | Source | Events | Recovery median [min–max] ms | Election median [min–max] ms | Ops in fault window ok / fail / indeterminate |
|---|---|---|---|---|---|
| RaftKV · kill-leader | E1 histories | 60 | 1,187 [1,012–1,762] | 1,168 [998–1,739] | 202,701 / 363 / 720 |
| etcd · kill-leader | E2 histories | 60 | 1,140 [942–1,623] | not observable | 168,355 / 641 / 2,020 |
| RaftKV · partition-leader | E1 histories | 60 | 1,504 [986–2,059] | 1,255 [939–1,885] | 194,154 / 88 / 499 |
| etcd · partition-leader | E2 histories | 60 | 1,219 [900–2,836] | not observable | 173,480 / 569 / 1,695 |
| RaftKV · kill-leader | new loadgen failover, 500 writes/s | 10 | 1,269 [1,083–1,594] | – | restarted node catch-up 308 [195–496] ms |
| etcd · kill-leader | new loadgen failover, 500 writes/s | 10 | 1,147 [1,004–1,440] | – | restarted node catch-up 422 [309–533] ms |

- **Election vs client-visible recovery (RaftKV):** under kill-leader, clients see writes again about 20 ms after the new leader is elected (medians 1,187 vs 1,168 ms). Under partition-leader the gap is about 250 ms (1,504 vs 1,255 ms), because clients homed on the isolated old leader must time out (1 s op timeout) before retrying elsewhere.
- **Back to normal operation** (successful-write rate back to ≥ 80% of the pre-fault rate for 1 s, in 250 ms bins): RaftKV median 1.75 s (kill) and 1.5 s (partition); etcd 3.5 s and 2.0 s. This metric depends on the harness clients' retry behaviour, so treat it as indicative.
- **Recovery consistency:** all 80 E1/E2 trials in these two scenarios were linearizable with 0 stale reads. For etcd partition-leader, 2 Unknown results and 1 checker crash were shown linearizable offline in the E1/E2 report.
- etcd recovers slightly faster at the median (about 50 ms for kill, 280 ms for partition), but the ranges overlap heavily. Both are dominated by the 1 s election timeout.
- Data: `analysis/e4-reanalysis.csv`, `raw/e4-killleader/*/failover.jsonl`, histories and node logs in `research/e1-e2-2026-10-15/raw/` (unchanged).

## 5. Figures

1. **Throughput** (offered vs achieved, per mix; the 50/50 panel includes the overload runs). Both systems follow the diagonal up to 8k; RaftKV's collapsed rep is visible as an outlier dot in the write-only panel.
2. **Latency** (p50 and p99 per mix, all reps). RaftKV's p50 is lower everywhere; p99 lines cross.
3. **Overload** (50/50, 8k–14k: throughput, p99, failed requests, busiest node CPU). Both systems stay on the diagonal in these runs; RaftKV's leader runs at about 115% CPU against etcd's 70%.
4. **Ablation** (p50 and p99 per configuration; unsafe fsync-off drawn hollow). Batching off collapses at 8k.
5. **Recovery** (per-event recovery time per system and fault, plus RaftKV election times).

No points were removed. All repetitions are drawn.

## 6. Limitations

- All 5 nodes and the client share one 4 vCPU machine and one disk, and the host runs near CPU saturation from about 2k ops/s. Absolute latencies, and the RaftKV-vs-etcd ordering, may not carry over to a real cluster with network latency.
- Repetitions were reduced to 4 (E3 50/50 and write-only) and 3 (E7) to fit the time budget. p50 conclusions are robust (non-overlapping ranges at most points); p99 and p99.9 comparisons are preliminary.
- RaftKV 95/5 combines 5 reps from the E5 extension run (same commit, same method, earlier the same day) with 2 new reps. The new reps have somewhat lower p50 at 4k–8k (8k: 3.8 and 4.2 ms vs 4.6–5.2 ms earlier). Pooling widens RaftKV's range; its median p50 is below etcd's with either subset, though at 1k the ranges overlap.
- Latency mixes reads and writes in one histogram (loadgen limitation).
- The cause of the intermittent collapse is not established. Goroutine counts and fsync counts were not measurable without code changes.
- Partition-leader recovery comes from re-analysis of E1/E2 histories (25 clients, 1 s op timeout, 2 active keys), not from a new dedicated run. Its RaftKV election times carry an estimated ±50 ms anchor uncertainty.
- etcd election time is not observable at the configured log level.
- The harness snapshot threshold (256 KB) is much smaller than RaftKV's default (64 MB) and raises disk traffic. It was kept unchanged so results stay comparable with earlier runs.

## 7. Assessment of RQ2 and recovery

- **RQ2 (cost of mechanisms, comparison with etcd).** There is enough evidence for a qualified answer:
  - Read path: lease < ReadIndex < log (E5).
  - Batching is essential for write throughput and stability; fsync roughly doubles p50 (unsafe comparison only); pipelining gives no benefit on a single host (E7).
  - Against etcd, RaftKV has lower median latency at all tested loads up to 8k and comparable p99, but **lacks backpressure** and can collapse at high write load where etcd degrades gracefully.
- **Recovery.** Both systems recover from a leader crash in about 1.1–1.3 s, set by the 1 s election timeout. Leader isolation costs RaftKV clients about 250 ms more than the election itself because of client timeouts. Consistency held in every recovery.

## 8. Hypotheses

- **H1** (lease reads break under clock skew; ReadIndex does not): **supported** (E5: 0/60 at 0.92×, 3/3 stale at 0.25×; ReadIndex clean in all conditions).
- **H2** (partitions expose more injected bugs than crashes): not addressed here; it remains **unresolved** pending the full E6 matrix (limited E6 detected ackbeforefsync under crashes and localread under partitions).
- **H3** (the etcd gap is mostly batching + fsync): **not supported as stated.** There is no p50 gap: RaftKV is faster at p50. Batching and fsync are the dominant internal costs, but they do not explain etcd's lower p99 in the 50/50 mix or its robustness under overload. The missing backpressure and higher disk traffic are the observed differences.

## 9. Is there enough evidence for an 8–12 page paper?

**Yes, for a paper scoped to RQ1 and RQ2 on a single host.** RQ1 (G1, E1, E2, E5) and RQ2 (E3, E5, E7, E4) have complete or clearly-labelled reduced results with figures. The paper should present the single-host setup, the reduced repetitions and the RaftKV overload collapse as limitations. RQ3 (bug detection, H2) rests only on the limited E6 and would be the weakest section.

## Files

- `analysis/`: scripts (`e3.sh`, `e3b.sh`, `overload.sh`, `e7.sh`, `e4.sh`, `chain2.sh`, `lib.sh`, `sampler2.py`, `aggregate.py`, `e4_reanalyse.py`), outputs (`bench.md`, `bench-per-rep.csv`, `bench-summary.json`, `e4-reanalysis.csv`), run logs.
- `raw/`: `e3/`, `overload/` (including `diag-goroutines-12000/` with the dumps in `n3.log` and `n4.log`), `e7/`, `e4-killleader/`, `attempts.log`. Each run has its command, config, console, HDR percentiles, node logs and 1 s resource samples.
- `figures/`: `rq2-figures.html`, `template.html`, `build.py`.
