# E5 extension: lease near the drift boundary, and read-path benchmarks

Date: 2026-10-16. Commit: `14bf0f6` (GitHub main, re-verified at start). No Raft core, checker or configuration changes were made.
Figures: https://claude.ai/artifact/PPPAkagCskSpkM39FHKSUo (source in `figures/`).

## Summary

- **Lease under mild drift (0.92×), 60 trials:** 0 violations. 20 Ok, 38 Ok (reduced), 2 Unknown, 0 Illegal, 0 stale reads. Both Unknown trials were shown linearizable offline. Combined violation rate 0/60, Wilson 95% [0%, 6.0%]. This is consistent with H1 but is not a proof of safety.
- **Benchmarks, 40 runs:** every run reached its offered load with 0 errors, 0 timeouts and 0 dropped requests. Lease has the lowest p50 and p99 at every load and in both mixes, and its advantage grows with load (p50 vs ReadIndex 1.3× at 1,000 ops/s up to 6.8–9.1× at 8,000 ops/s).
- **Outliers:** the pilot's 181 ms log p99 at 1,000 ops/s did not recur (p99 6.9–12.6 ms in 5 repetitions). Snapshots are not associated with tail spikes. GC pauses explain some lease p99.9 spikes at 8,000 ops/s but not all. CPU contention on the 4-vCPU host is the main confound for log and ReadIndex tails.
- **Recommendation:** E5 is sufficient for the paper's safety claim about lease reads (with the stated bound) and for the p50/p99 comparison. p99.9 comparisons should be reported as preliminary.

## 1. The 60 lease / mild-drift trials

Configuration (identical to the pilot): read path lease, fault `clock-partition-mild` (leader clock slowed to 0.92× and leader partitioned), 5 nodes, 34 s per trial, 3 s warmup, a fault every 7 s lasting 5 s (3 faults per trial), 25 clients, key rotation every 12 s, 2 active keys, 1 s op timeout, unique values, election timeout 1 s, heartbeat 100 ms, checker timeout 60 s, one trial per harness process. Script: `analysis/part1.sh`.

| Outcome | Trials |
|---|---|
| Ok | 20 |
| Ok (reduced) | 38 |
| Unknown | 2 (attempts 1 and 12) |
| Illegal | 0 |
| Checker crash / OOM | 0 |
| Trials with provably stale reads | 0 (0 stale reads in total) |

- Faults: 180 planned, 180 executed, 180 completed (3/3/3 in every trial). Actual durations 5.069–5.127 s.
- Peak checker memory (RSS): median 2.97 GB, maximum 10.01 GB.
- The offline stale-read scan (`analysis/aggregate_part1.py`) agreed with the harness in every trial.
- Per-trial table (verdict, unknown keys, stale reads, faults, durations, peak RSS, wall time): `analysis/part1.md`. Raw histories, nemesis logs and node logs: `raw/lease-mild/attempt-NNN/`.

**Interruption.** The container restarted while attempt 60 was being checked. Its history and nemesis log had already been written; they are kept in `raw/lease-mild-interrupted/attempt-060/`, and an offline stale scan found 0 stale reads in its 32,129 operations. Attempt 60 was then rerun from scratch and came back Ok. Only the rerun is counted. The event is recorded in `raw/attempts.log`.

## 2. Confidence intervals and inconclusive outcomes

| Metric | Count | Rate | Wilson 95% |
|---|---|---|---|
| Combined (Illegal or stale read), all 60 trials | 0/60 | 0% | 0% – 6.0% |
| Porcupine-only (Illegal), Unknown excluded | 0/58 | 0% | 0% – 6.2% |

The rule of three gives a similar upper bound (about 5%). Zero violations in 60 trials means the per-trial violation probability under this fault is below about 6% with 95% confidence. It does not show it is zero.

**The two Unknown trials** (`analysis/recheck/recheck.log`):
- Attempt 1, key 2 (4,688 ops): the full check hit the 60 s limit (Unknown). The reduced check returned Illegal, which is not proof, because the reduction drops 20 indeterminate writes that could explain a failed CAS. Adding back 1 of those writes gave Ok, so the history is **linearizable**.
- Attempt 12, key 1 (5,102 ops): the per-key full check returned Ok in under 1 s. **Linearizable.**

Both had 0 stale reads. After the recheck, all 60 histories are known to be linearizable; the Porcupine-only rate above still counts them as Unknown, as the methodology requires.

## 3. Benchmarks

Method (same as the pilot): `loadgen`, open loop, 5 nodes, no faults, 10,000 keys, 128 B values, 5 s warmup, 20 s measured window, offered loads 1,000 / 2,000 / 4,000 / 8,000 ops/s, mixes 100% reads and 95% reads / 5% writes, 5 independent repetitions per path and mix. Path order was rotated in each repetition. A 1 s sampler recorded CPU, RSS and CPU pressure (PSI) for every node and loadgen, and snapshot file changes. Script: `analysis/part2.sh`.

Main values are medians over 5 repetitions; brackets give min–max. Latency in ms, measured from each request's intended start time.

**95% reads / 5% writes**

| Path | Load | p50 | p99 | p99.9 | Node CPU (sum) |
|---|---|---|---|---|---|
| log | 1,000 | 2.08 [2.02–2.34] | 10.86 [8.43–23.20] | 28.22 [15.85–116.29] | 242% |
| log | 2,000 | 3.17 [3.05–3.35] | 14.31 [11.78–25.45] | 33.28 [26.03–73.41] | 252% |
| log | 4,000 | 4.80 [4.30–5.07] | 31.44 [19.38–38.69] | 45.38 [33.66–92.03] | 248% |
| log | 8,000 | 8.43 [7.93–9.62] | 71.30 [56.64–94.66] | 103.81 [97.28–190.21] | 234% |
| ReadIndex | 1,000 | 1.13 [1.11–1.25] | 3.81 [3.55–5.36] | 10.57 [6.59–48.77] | 177% |
| ReadIndex | 2,000 | 1.21 [1.07–1.40] | 7.36 [6.11–60.48] | 26.38 [16.18–235.13] | 244% |
| ReadIndex | 4,000 | 2.47 [2.19–2.65] | 11.07 [9.88–13.83] | 23.76 [18.22–38.43] | 255% |
| ReadIndex | 8,000 | 5.04 [4.62–5.24] | 22.62 [19.50–32.54] | 40.58 [30.83–52.51] | 242% |
| lease | 1,000 | 0.89 [0.88–0.91] | 3.05 [2.73–3.15] | 7.43 [5.01–18.94] | 67% |
| lease | 2,000 | 0.64 [0.63–0.65] | 3.35 [2.99–4.70] | 10.77 [7.06–64.35] | 96% |
| lease | 4,000 | 0.42 [0.41–0.52] | 3.96 [3.64–6.46] | 33.12 [11.65–51.42] | 142% |
| lease | 8,000 | 0.74 [0.73–0.80] | 8.70 [8.00–18.62] | 21.54 [17.71–41.41] | 182% |

**100% reads**

| Path | Load | p50 | p99 | p99.9 | Node CPU (sum) |
|---|---|---|---|---|---|
| log | 1,000 | 2.06 [1.95–2.31] | 10.26 [6.93–12.62] | 29.41 [14.90–38.30] | 242% |
| log | 2,000 | 3.27 [2.88–3.42] | 17.57 [10.41–28.30] | 41.76 [21.92–51.01] | 252% |
| log | 4,000 | 4.61 [4.27–4.92] | 28.27 [17.12–38.11] | 56.77 [30.64–65.38] | 248% |
| log | 8,000 | 7.31 [6.32–8.57] | 52.77 [46.37–58.34] | 76.22 [67.33–88.45] | 238% |
| ReadIndex | 1,000 | 1.08 [1.05–1.08] | 3.27 [3.06–3.52] | 6.88 [5.48–7.27] | 170% |
| ReadIndex | 2,000 | 0.93 [0.93–0.97] | 4.74 [4.04–5.65] | 12.63 [7.27–80.19] | 252% |
| ReadIndex | 4,000 | 1.63 [1.50–1.66] | 7.54 [7.39–12.35] | 11.24 [10.89–102.33] | 271% |
| ReadIndex | 8,000 | 3.17 [2.97–3.43] | 12.81 [12.26–35.68] | 22.34 [16.72–118.02] | 258% |
| lease | 1,000 | 0.85 [0.84–0.90] | 1.94 [1.80–41.98] | 4.71 [3.20–153.09] | 46% |
| lease | 2,000 | 0.58 [0.56–0.59] | 2.35 [2.20–2.48] | 12.70 [6.95–41.09] | 59% |
| lease | 4,000 | 0.36 [0.35–0.37] | 1.71 [1.67–2.39] | 4.83 [3.38–80.06] | 96% |
| lease | 8,000 | 0.35 [0.34–0.36] | 2.30 [1.92–2.62] | 11.62 [4.09–23.45] | 114% |

- Throughput: achieved = offered in all 40 runs (median and every repetition). Errors, timeouts and drops: 0 everywhere (loadgen counts timeouts as errors).
- **Write latency is not reported separately.** loadgen keeps one histogram for all operations, so the 95/5 numbers cover reads and writes together. Separating them would need a loadgen change, which was not made.
- Memory: maximum node RSS 40.8 MB (log), 38.7 MB (ReadIndex), 36.3 MB (lease).
- CPU pressure (PSI "some" avg10, maximum per point, median over points): log 64%, ReadIndex 64%, lease 25%. The host has 4 vCPU shared by 5 nodes and loadgen, so log and ReadIndex run under heavy CPU contention. Log uses about 240% CPU at every load, even 1,000 ops/s; the reason was not investigated.
- Full per-repetition data: `analysis/bench-per-rep.csv`; aggregated table: `analysis/bench.md`; figure data: `analysis/bench.json`.

## 4. Comparisons and figures

Figure A (p50 and p99 vs load), Figure B (p99.9 with per-repetition spread) and Figure C (lease safety with Wilson intervals) are in the artifact linked at the top, drawn only from the files above.

**How much faster is lease?** Ratio of medians (ReadIndex / lease, log / lease):

| Mix | Load | p50 | p99 |
|---|---|---|---|
| 95/5 | 1,000 | 1.3× / 2.3× | 1.2× / 3.6× |
| 95/5 | 2,000 | 1.9× / 4.9× | 2.2× / 4.3× |
| 95/5 | 4,000 | 5.9× / 11.4× | 2.8× / 7.9× |
| 95/5 | 8,000 | 6.8× / 11.4× | 2.6× / 8.2× |
| 100% | 1,000 | 1.3× / 2.4× | 1.7× / 5.3× |
| 100% | 2,000 | 1.6× / 5.7× | 2.0× / 7.5× |
| 100% | 4,000 | 4.5× / 12.9× | 4.4× / 16.5× |
| 100% | 8,000 | 9.1× / 20.9× | 5.6× / 23.0× |

**Does the advantage hold across loads?** Yes, at p50 and p99 it holds at every load and grows with load, because log and ReadIndex latency rises with load while lease stays flat (100% reads) or rises slowly (95/5, where the 5% writes still go through the log). Part of the growth reflects CPU contention: lease uses far less CPU, so it also suffers less from the shared 4-vCPU host. The absolute ratios are therefore specific to this host.

**Which comparisons are statistically reliable?** With 5 repetitions, a point counts as clearly separated when every repetition of one path is below every repetition of the other.
- p50: lease < ReadIndex < log at all 8 load/mix points. **Reliable.**
- p99: separated at 7 of 8 points for each pair. The exceptions are lease vs ReadIndex/log at 100% reads, 1,000 ops/s (one lease repetition had p99 = 42 ms) and ReadIndex vs log at 95/5, 2,000 ops/s. **Reliable, with these exceptions noted.**
- p99.9: lease vs ReadIndex is separated at 0 of 8 points, and log vs the others at 2 of 8. **Preliminary.** The median p99.9 favours lease at most points, but the repetition spread is too large to claim an ordering.

**Does the mild-clock result support H1?** Yes, within its limits. H1 says lease reads are safe while clock drift stays inside the configured bound and unsafe beyond it. At 0.92× the lease ends about 0.978 s of real time after the quorum ack, just before followers can vote at 1 s (a margin of about 2%). 0 violations in 60 trials, against 3/3 trials with stale reads at 0.25× in the pilot, fits H1. The 0.25× point is a 3-trial reference only and is not pooled.

**Lease is not a free optimization.** Its speed comes from serving reads without contacting a quorum, which is safe only under bounded clock drift. Configurations with drift outside the bound (as in the 0.25× pilot) or with the lease bound disabled (the `leasenodrift` bug variant from E6) are unsafe and must not be presented as performance options.

## 5. Outlier analysis

Data: per-repetition sampler windows (`analysis/bench-per-rep.csv`) and 10 extra diagnostic runs with `GODEBUG=gctrace=1` (`analysis/diag-per-rep.csv`, `raw/diag-gctrace/`). The diagnostic runs have GC tracing on, so they are used only to explain outliers and are not pooled with the main results. No repetition was discarded.

**Log p99 at 1,000 ops/s (100% reads).** The pilot saw 181.6 ms in one of two repetitions. Here the five main repetitions gave 6.9–12.6 ms, so the large value did not recur. In the 5 diagnostic runs, p99.9 reached 138.6 ms in one run, but the longest GC stop-the-world pause in any of those windows was 2.8–7.1 ms, so GC does not explain it. Every log window contains about 15 snapshot writes, so snapshots are present in good and bad repetitions alike. All log runs show CPU pressure of about 56–58%. Our inference is that the log tail is driven by CPU scheduling contention; this was not proven.

**Lease p99.9 at 8,000 ops/s (100% reads).** The pilot ranged 11.7–52.5 ms; the main runs here ranged 4.1–23.5 ms (median 11.6). No snapshot was written in any lease window. In the diagnostic runs, p99.9 was 43.1, 37.6, 16.9, 6.5 and 34.1 ms, and the longest GC pause was 39.0, 2.1, 5.0, 3.0 and 29.0 ms. GC pauses of 29–39 ms match the two runs with the highest tails, but the second-highest (37.6 ms) had only a 2.1 ms pause, so GC explains part of the tail, not all of it. loadgen used about 115% CPU at this load, so the load generator itself is a likely contributor.

**Snapshots across all runs.** p99.9 relative to its point's median: with a snapshot in the window, median 1.00, mean 1.35 (n = 79); without one, median 1.00, mean 3.02 (n = 41). Tail spikes are not associated with snapshots.

**Other large spikes** (p99.9 more than 3× the point median, 14 of 120 repetitions): 7 are ReadIndex runs with CPU pressure of 49–70%, 6 are lease runs with no snapshots and lower pressure (10–38%), and 1 is a log run. No single cause covers them; they are kept in all statistics.

## 6. Limitations

- 0/60 is not a proof of safety. The 95% upper bound on the per-trial violation rate is 6.0%; rarer failures would not be detected.
- Only one mild drift level (0.92×) was tested; the boundary between 0.92× and 0.25× is unmapped. The normal and severe points have only 3 trials each.
- The safety argument depends on the configured drift bound (10%) and the 1 s minimum election timeout. Other settings were not tested.
- Single host with 4 vCPU running all 5 nodes and the load generator. CPU contention is substantial for log and ReadIndex and affects their tails and the size of the speed-up. Results do not transfer directly to a real multi-machine cluster with network latency.
- Write latency is not separated from read latency in the 95/5 mix.
- 5 repetitions are enough for p50 and p99 comparisons but not for p99.9.
- loadgen was itself near one full CPU at 8,000 ops/s for lease, so the highest-load lease tails may include client-side delay.
- The resource sampler and log tailing ran during the benchmarks; their overhead looks small (log p50 at 1,000 ops/s was 2.06 ms here vs 1.76 ms in the pilot) but was not measured separately.
- Checker memory reached 10 GB in one trial; larger histories could exceed the container's limit.

## 7. Is E5 now sufficient for the paper?

**For the safety claim: yes, with the bound stated.** The paper can say that lease reads were linearizable in 60/60 trials at 0.92× drift (95% upper bound 6.0% per trial), that log and ReadIndex were clean in all pilot conditions, and that lease reads became stale in 3/3 trials at 0.25× drift. The last point should be presented as a demonstration, not a rate estimate.

**For the performance claim: yes for p50 and p99, preliminary for p99.9.** The ordering lease < ReadIndex < log is robust at p50 (8/8 points) and p99 (7/8), and the advantage grows with load. p99.9 should be shown with its spread (Figure B) and described as preliminary, and the CPU contention of the test host should be stated.

Optional work that would strengthen E5 (not run, needs your approval): more trials at 0.92× or a second drift level near the boundary (for example 0.85×) to map where safety ends; more repetitions or a larger host for p99.9; a loadgen change to report write latency separately.

## Files

- `analysis/`: scripts (`part1.sh`, `part2.sh`, `chain.sh`, `sampler.py`, `aggregate_part1.py`, `aggregate_bench.py`), outputs (`part1.md`, `bench.md`, `bench.json`, `bench-per-rep.csv`, `diag-per-rep.csv`), run logs (`part1.log`, `part2.log`), offline recheck (`recheck/`).
- `raw/`: `lease-mild/` (60 trials), `lease-mild-interrupted/` (first attempt 60), `speed/rf{1.0,0.95}/{log,readindex,lease}/rep-N/`, `diag-gctrace/`, `attempts.log`. 1,238 files, 256 MB.
- `figures/`: `e5-figures.html` (Figures A–C), `template.html`, `data.json`.
- Earlier results (`g1-e6-baseline`, `e6-limited-2026-10-15`, `e1-e2-2026-10-15`, `e5-pilot-2026-10-16`) were not modified.
