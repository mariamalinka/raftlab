# E5 pilot: three read paths under clock faults (2026-10-16)

## Summary

- **Correctness:** 27 trials, 3 per configuration.
  - Log reads and ReadIndex were linearizable in all 18 of their trials, under all three clock conditions.
  - Lease reads were linearizable at normal speed and at the mild 0.92× slowdown (6 of 6 trials).
  - At the severe 0.25× slowdown, lease reads returned **provably stale reads in 3 of 3 trials (75 reads)**. Porcupine returned Unknown for all 3.
- **Cause:** every stale read was served by the isolated old leader, the node whose clock was slowed, between 1.5 and 3.4 s after the fault started. This is the window in which a 0.25× clock makes its lease outlast the election of a new leader.
  - This is the expected result of deliberately breaking the lease's clock-drift assumption, not an implementation defect.
- **Performance (no faults, same offered load):** lease is the fastest path, then ReadIndex, then log reads, at every load tested.
  - At 8,000 ops/s of pure reads, median latency was 0.31 ms for lease, 2.48 ms for ReadIndex and 5.87 ms for log reads.
  - No configuration saturated, and no measurement comes from a run with anomalies.
- **H1** (lease reads break under clock skew, ReadIndex doesn't) is supported by this pilot. The pilot is too small to put tight bounds on it.
- **The full E5 is ready**, with the changes in section 6.

## 1. Baseline

- **Code:** GitHub `main` at `14bf0f66850fafb1418c6daf77c9c8b237c915b5`, your E1/E2 results commit, read from a clean clone of your device's `.git`.
  - Its code is identical to `80f06fc`: `git diff 80f06fc 14bf0f6` touches only `results/`.
  - `git status` on the clone is clean.
  - The R-1 fix, `00d9651`, is present.
- **Nothing in the repository was changed.** Binaries were built from that commit, and `make bugtests` gave 7/7 caught.
- **Earlier results are untouched.** The E1, E2, limited E6 and G1 results are in git and in `research/`.
- **The three read paths** are selected with `-read-mode` on the node (`cmd/raftkv`), and the harness passes it through (`internal/kv/store.go` 366–395):
  - **log:** the Get goes through the Raft log like a write.
  - **readindex:** the leader confirms its leadership with a heartbeat round, then reads at the commit index.
  - **lease:** if the leader's lease holds, it reads at once without a heartbeat round; otherwise it falls back to ReadIndex. The lease lasts election timeout × (1 − 0.1) on the leader's own clock (`raft.go` 1025–1043), measured from the time the leader sent heartbeats that a quorum acknowledged.
  - A follower refuses to vote for a new leader until a full election timeout has passed since it last heard from the leader.

## 2. Design

Every trial isolates the current leader, so the three conditions differ only in the isolated leader's clock rate. Each trial is its own process. The settings are the E6H line used for E1 and E2: 5 nodes, 25 clients, 34 s, faults at 3, 15 and 27 s lasting 5 s, 12 s key epochs, unique values, and a 1 s election timeout.

| Condition | Fault | Leader clock | What lease safety predicts |
|---|---|---|---|
| normal | partition-leader | 1.00× | safe |
| mild | clock-partition-mild | 0.92× | safe. The lease ends after 0.9/0.92 = 0.98 s of real time, before followers may vote at 1 s, a 2% margin. |
| severe | clock-partition | 0.25× | unsafe. The lease lasts 0.9/0.25 = 3.6 s of real time, while followers elect a new leader after 1 to 2 s. |

## 3. Correctness

| Read path | Condition | Ok | Ok (reduced) | Unknown | Illegal | Checker crash | Trials with stale reads | Stale reads | Median first stale read | Faults p/e/c |
|---|---|---|---|---|---|---|---|---|---|---|
| log | normal | 3 | 0 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| log | mild | 2 | 1 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| log | severe | 2 | 1 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| readindex | normal | 3 | 0 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| readindex | mild | 2 | 1 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| readindex | severe | 3 | 0 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| lease | normal | 1 | 2 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| lease | mild | 1 | 2 | 0 | 0 | 0 | 0/3 | 0 | – | 3/3/3 |
| **lease** | **severe** | 0 | 0 | **3** | 0 | 0 | **3/3** | **75** (27, 21, 27) | 5.1 s | 3/3/3 |

- All 81 faults ran for their full duration: the shortest lasted 5.08 s, with planned = executed = completed = 3 in every trial.
- There were no checker crashes. Peak checker memory was 12.2 GB on lease/severe and at most 5.4 GB elsewhere.
- No configuration had a Porcupine Illegal verdict, so there is no median Porcupine detection time.
- With 3 trials, a 0/3 cell has a 95% upper bound of 0.56, and a 3/3 cell a lower bound of 0.44.

### The lease/severe trials

**The original verdicts stay as recorded:** Unknown, Unknown, Unknown. The stale-read scan is reported separately.

**Why the stale reads are real.** The scan only flags a read whose value no write could have produced. This happens when a newer write had already returned to its client before the read was invoked. All 75 flagged reads share one pattern (`analysis/stale-windows.txt`):
- They were served by the slowed, isolated leader: the client's home node was the node whose clock the fault slowed.
- They occurred inside the fault window, 1.52 to 3.35 s after it started. Not one occurred outside a fault window.
- That window runs from the new majority leader's first committed writes (about 1.5 s) to the end of the slowed lease (about 3.6 s of real time).

**Offline Porcupine re-check** (`analysis/offline-recheck/`):
- Each key was checked on its own, with a 60 s limit. The keys without stale reads came out Ok.
- Every key with stale reads stayed Unknown, even when cut off right after its first stale read.
- The reduced check gives Illegal on exactly the keys the scan flags. As in E2, a reduced Illegal does not count as proof.
- **So Porcupine could not confirm these violations. The evidence is the stale-read scan alone**, which is sound but conservative.

**Implementation defect or assumption violation?** An assumption violation.
- At 0.25×, the leader's clock runs 75% slow, far past the 10% drift bound that leases are built to tolerate. Inside that bound, at 0.92×, there were no anomalies.
- ReadIndex and log reads, which do not depend on clocks, stayed correct under the same 0.25× fault.
- This matches the October 9 pilot (3/3 Illegal under clock-partition with 5 s key rotation). It does not match the limited E6 `leasenodrift` bug, which breaks leases inside the bound.

## 4. Preliminary performance

Load came from the open-loop `loadgen` with no faults, 5 nodes, 10,000 keys, 128 B values, 20 s measured after a 5 s warmup, and 2 repetitions. Every read path got the same offered loads. Successful ops/s equalled offered load everywhere, with 0 errors and 0 dropped requests. Values are the mean of the 2 repetitions.

**Reads only (read fraction 1.0):**

| Offered (ops/s) | log p50 / p99 / p99.9 (ms) | readindex p50 / p99 / p99.9 | lease p50 / p99 / p99.9 |
|---|---|---|---|
| 1,000 | 1.76 / 93.9* / 128.9* | 1.02 / 2.70 / 7.07 | 0.82 / 1.67 / 3.28 |
| 4,000 | 3.68 / 22.6 / 37.9 | 1.26 / 6.66 / 13.0 | 0.32 / 1.64 / 4.09 |
| 8,000 | 5.87 / 40.8 / 51.8 | 2.48 / 10.0 / 14.0 | 0.31 / 2.00 / 32.1* |

**95% reads, 5% writes:**

| Offered (ops/s) | log | readindex | lease |
|---|---|---|---|
| 1,000 | 1.79 / 7.38 / 18.3 | 1.10 / 3.33 / 9.62 | 0.84 / 2.56 / 5.53 |
| 4,000 | 3.75 / 26.3 / 42.6 | 1.71 / 8.26 / 18.5 | 0.37 / 2.82 / 8.36 |
| 8,000 | 6.19 / 48.2 / 62.8 | 3.91 / 15.5 / 25.3 | 0.50 / 5.06 / 11.4 |

\* High variance between the two repetitions: log at 1,000 ops/s ranged from 6.2 to 181.6 ms for p99, and lease at 8,000 ops/s from 11.7 to 52.5 ms for p99.9. These are probably isolated stalls in one repetition, perhaps a snapshot or GC pause, but that is inferred, not checked.

Lease p50 is also *lower* at 4,000 ops/s than at 1,000 (0.32 against 0.82 ms). This most likely comes from idle wakeups at low load, a measurement effect to check before E5. Per-repetition values and HDR percentile files are in `raw/speed/`.

**These numbers come from fault-free, linearizable runs only.** The latency measured for leases is real, but it is safe only while clocks stay inside the drift bound. Section 3 shows what happens outside it. The latency tables are a basis for the E5 figures, not benchmark conclusions. There were 2 repetitions, no load high enough to saturate, and one 4-vCPU machine.

## 5. Evaluation

1. **Correctness across read paths:** log = ReadIndex = lease under normal and mild clocks: 0 anomalies in 3 trials each. Only lease under 0.25× broke, in 3/3 trials.
2. **H1:** supported. Lease reads broke when the clock assumption was violated, and ReadIndex did not, under the identical fault. The pilot cannot yet bound the rates tightly.
3. **Configurations that need more investigation:**
   - **lease × mild:** the safety margin is only 2%, and 3 trials say little. This is the cell where a real lease defect would show.
   - **lease × severe:** Porcupine is no help here (Unknown in 3/3, 12 GB), so the result rests on the stale-read scan alone.
   - **Performance outliers:** the log p99 at 1,000 ops/s and the low-load lease p50 effect.
4. **Is the full E5 ready?** Yes, with these settings:
   - one trial per process;
   - combined detection (Illegal or a stale read) as the primary metric, as decided for E6;
   - a longer performance sweep.
5. **Minimum additional trials:**
   - **Correctness:** 20 per cell (9 cells, 180 trials, about 3 hours), as in the plan, gives each 0/20 cell an upper bound of 0.16.
   - **lease × mild:** 60 trials, for a 95% upper bound of 0.05 on the violation rate if all pass.
   - **Performance:** 5 repetitions per point, with offered loads from 1,000 up to past saturation (for example 1k, 2k, 4k, 8k, 16k, 24k, 32k) at read fraction 0.95, plus one 1.0 sweep.
6. **Proposed final E5 figures:**
   - **Figure A, latency vs offered load:** p50 and p99 lines per read path on log axes, with read fraction 0.95, fault-free and linearizable runs only. This is the plan's headline chart.
   - **Figure B, correctness matrix:** 3 read paths × 3 clock conditions. Each cell shows the combined violation rate with its Wilson CI, plus small Unknown and crash counts. The severe column is marked "outside the drift bound".
   - **Figure C, stale-read timeline:** stale reads against time since fault start, for lease × severe. It shows reads clustered between the new leader's election and the predicted end of the lease (3.6 s), which explains the mechanism.
   - **Figure D, cost vs safety summary:** p99 latency at a fixed load (for example 4,000 ops/s) plotted against the violation rate under the severe clock, one point per read path, with leases labelled "safe only under bounded drift".

## Reproduce

On Linux, as root, with `iproute2` and Go 1.26, at commit `14bf0f6`:

```
export BIN=/tmp/raftkv-bin-e5 && make build && make bugtests
bash analysis/run.sh                                  # 27 correctness trials, then the speed runs
python3 analysis/aggregate.py raw > analysis/RESULTS.md
python3 analysis/stale_windows.py raw/correctness/lease/severe/attempt-00*/trial-001
```

## Files

Everything is under `research/e5-pilot-2026-10-16/`, 109 MB in total.

- **`raw/correctness/<path>/<condition>/attempt-NNN/`:**
  - `command.txt`, `console.log`, `summary.json` (configuration);
  - `trial-001/` with `history.jsonl`, `nemesis.jsonl`, `n1..n5.log`, `result.json`.
- **`raw/speed/rf<fraction>/<path>/`:** `command.txt`, `config.json`, `points.jsonl`, and an HDR percentile file per point.
- **`raw/attempts.log`:** exit code, wall time and peak RSS per trial.
- **`analysis/`:**
  - `RESULTS.md` (every table, plus each trial);
  - `aggregate.py`, `run.sh`, `stale_windows.py`, `stale-windows.txt`;
  - `offline-recheck/`.
