# E1 and E2: RaftKV correctness and the etcd control (2026-10-15)

## Summary

- **E1 (RaftKV, bug-free build, ReadIndex reads):** 180 trials across 9 fault scenarios.
  - All 180 were linearizable: 147 Ok, plus 33 Ok (reduced).
  - There were 0 Unknown, 0 Illegal, 0 provably stale reads and 0 checker crashes.
- **E2 (etcd v3.7.2, linearizable reads):** 160 trials across 8 matching scenarios.
  - 156 were linearizable as judged by the harness: 68 Ok, plus 88 Ok (reduced).
  - 3 were Unknown and 1 was a checker crash (out of memory). There were 0 Illegal results and 0 provably stale reads.
  - An offline per-key re-check shows that all 4 inconclusive etcd histories are linearizable (section 4). These offline results are reported separately and do not change the harness verdicts in the tables.
- **No safety violation was found** in either system, so the stop rule never fired.
- **RQ1:** the evidence supports the claim that RaftKV's ReadIndex configuration stayed linearizable under every tested fault. This is not a proof of correctness (section 6).
- **Recommendation:** the project is ready to move to E5. Two methodology caveats are listed in section 7.

## 1. Repository and environment

- **Code:** GitHub `main` at `80f06fc1d52cd2dec170c07dc77cfab71a4fc9c5`. I read it from a clean clone of your device's `.git`, where `refs/remotes/origin/main` points to this commit. `git status --porcelain` on the clone is empty. I could not see uncommitted edits in your Windows working tree itself.
- **R-1 fix present:** `00d9651` "raft: follower acknowledges AppendEntries only after its log is stored".
- **Latest work present:**
  - harness and checker: `31aea1c`, `3055bd5`;
  - E6 and analysis: `a2ffbb4`, `30e86b0`, `85a3e22`;
  - results: `80f06fc`.
- **Nothing was reapplied or modified in the repository.** Binaries were built from that commit (`make build`), and `make bugtests` gave 7/7 caught.
- **Machine:** Go 1.26.0 linux/amd64, 4 vCPU, 16 GB RAM, with a container memory limit of about 13.8 GB. Runs took place from 10:20 to 19:37 UTC.

## 2. Method

Every trial uses the E6H line of `scripts/experiment.sh`, the same settings as the limited E6:

- 5 nodes and 25 clients;
- `-time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s`;
- `-key-rotate 12s -unique-values`;
- election timeout 1 s and heartbeat 100 ms for both systems.

Faults start at 3, 15 and 27 s, so 3 are planned per trial. Each trial is a fresh cluster and its own `rkharness -trials 1` process, which means a checker crash costs only that trial.

The harness writes `history.jsonl` and `nemesis.jsonl` before Porcupine runs. If the process dies while checking, the run script copies the node logs from its data directory (`analysis/run.sh`). The run would stop as soon as any trial was Illegal or had a provably stale read. That never happened.

| Scenario | E1 RaftKV | E2 etcd | Difference |
|---|---|---|---|
| kill-leader, kill-majority, pause-leader, partition-leader, partition-halves, partition-bridge | yes | yes | identical |
| kill-all | with `-lazyfs` (a killed node loses unsynced WAL writes) | **no LazyFS** | LazyFS here is a RaftKV WAL mode. etcd fsyncs its WAL and a SIGKILL keeps the page cache, so etcd's kill-all is a weaker test. |
| clock-skew | yes (in-process clock: jumps of up to ±2 election timeouts, rates from 0.25× to 4×) | **not run** | Clock faults need RaftKV's skewable clock. etcd has no equivalent. |
| mixed | yes | yes | The pool includes clock-partition, which etcd cannot run. 5 of 60 etcd rounds logged "fault not supported" and injected nothing. E1's 3 clock-partition rounds did run. |
| Reads | ReadIndex | linearizable (default, not serializable) | |
| Other | snapshots every 256 KiB | `--snapshot-count 10000` | Harness defaults, unchanged. |

**Outcome rules:**
- Ok and Ok (reduced) are correct histories.
- Unknown and checker crash are inconclusive. They are never counted as correct.
- Illegal, or a provably stale read from the conservative scan, is a violation.

## 3. Results per scenario

| Scenario | Trials E1 / E2 | Ok + Ok (red.) E1 | Ok + Ok (red.) E2 | Inconclusive E1 / E2 | Illegal E1 / E2 | Stale reads E1 / E2 | Checker crash E1 / E2 | Violation rate, 95% CI (both) |
|---|---|---|---|---|---|---|---|---|
| kill-leader | 20 / 20 | 16 + 4 | 19 + 1 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| kill-majority | 20 / 20 | 17 + 3 | 10 + 10 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| kill-all | 20 / 20 | 16 + 4 (LazyFS) | 16 + 4 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| pause-leader | 20 / 20 | 16 + 4 | 4 + 16 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| partition-leader | 20 / 20 | 14 + 6 | 2 + 15 | 0 / **3** (2 Unknown, 1 crash) | 0 / 0 | 0 / 0 | 0 / **1** | E1 0/20 [0.00, 0.16]; E2 0/19 [0.00, 0.17] |
| partition-halves | 20 / 20 | 14 + 6 | 1 + 19 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| partition-bridge | 20 / 20 | 16 + 4 | 8 + 12 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| clock-skew | 20 / – | 19 + 1 | – | 0 / – | 0 / – | 0 / – | 0 / – | E1 0/20 [0.00, 0.16] |
| mixed | 20 / 20 | 19 + 1 | 8 + 11 | 0 / **1** (Unknown) | 0 / 0 | 0 / 0 | 0 / 0 | 0/20 [0.00, 0.16] |
| **Total** | **180 / 160** | **147 + 33** | **68 + 88** | **0 / 4** | **0 / 0** | **0 / 0** | **0 / 1** | E1 0/180 [0.00, 0.02]; E2 0/159 [0.00, 0.02] |

The violation rate counts trials that were Illegal or had a stale read, over trials where the checker finished. The tables per cell, with operation counts, faults, wall time and peak memory, are in `analysis/RESULTS.md`, along with every trial.

### Client operations

| | Ops | ok | fail | info (indeterminate) |
|---|---|---|---|---|
| E1 | 5,499,649 | 5,469,642 | 9,772 | 20,235 (0.37%) |
| E2 | 4,447,350 | 4,405,476 | 12,929 | 28,945 (0.65%) |

- The most indeterminate operations, in both systems, came from kill-majority and kill-all (1.3% to 1.8%), where the cluster loses quorum.
- etcd had 1.3 to 3.3 times more indeterminate operations than RaftKV in the leader, pause and partition scenarios. For example, kill-leader was 0.43% against 0.14%, and pause-leader 0.37% against 0.29%.

### Fault execution

- **E1:** all 180 trials ran 3/3/3 (planned/executed/completed). The shortest fault lasted 5.00 s.
- **E2:** 156 trials ran 3/3/3. The exceptions were 4 mixed trials, at 3/2/2 ×3 and 3/1/1 ×1, all caused by the unsupported clock-partition draw. Every executed fault lasted at least 5.00 s, and there were no other fault errors.

## 4. Inconclusive etcd trials

The harness verdicts stay as they are: 3 Unknown and 1 crash. The work below is a separate offline analysis of the saved histories (`analysis/offline-recheck/`). It uses Porcupine and the harness's own `reduceKey` from test files outside the repository.

| Trial | Harness result | Offline finding |
|---|---|---|
| partition-leader / attempt-005 | checker crash: killed by the kernel's out-of-memory handler at 13.8 GB (`dmesg-tail.txt`) | Saved history: 0 stale reads. Each of its 8 keys checked one at a time is Ok (reduced) in under 1 s. **Linearizable.** |
| partition-leader / attempt-008 | Unknown (7 keys Ok (reduced), key 4 not) | Key 4: the full check runs out of memory at 13.9 GB. The reduced check is **Illegal**. Adding back 1 of the 22 dropped indeterminate writes makes it Ok. **Linearizable.** |
| partition-leader / attempt-019 | Unknown (key 2 not Ok after reduction) | Key 2 checked alone, without reduction, is Ok in under 1 s. Its reduced check is Illegal. **Linearizable.** |
| mixed / attempt-011 | Unknown (key 3) | Same pattern as attempt-008. Adding back 1 of 47 dropped writes gives Ok. **Linearizable.** |

**Why the reduction said Illegal, and why that is not an etcd bug.**
- The reduction drops indeterminate writes whose value no read observed.
- One of those writes can still have taken effect and made a later CAS fail. Dropping it leaves that failed CAS unexplained.
- In attempt-008 and attempt-011, adding back any one of 12 to 21 such writes explains the first failing prefix (`zz_explain_test.go`).
- The harness already treats a reduced check that is not Ok as proving nothing, so it reported Unknown. That was correct.
- The partial reduction used here is sound for Ok. It still drops all but one of the unobserved indeterminate writes, and each of those may be linearized as having had no effect.

## 5. Unexpected behaviour

- **Checker memory and time are much higher on etcd histories.** RaftKV's peak median was 0.03 to 0.33 GB per cell (3.8 GB at most), with a median trial time of 42 to 44 s. For etcd in the pause-leader, partition and mixed cells the peak median was 3.2 to 7.4 GB (13.8 GB at most), with a median of 103 to 106 s.
  - The cause is that more of etcd's histories hit the checker timeout and needed the reduced check: 88/160 for etcd against 33/180 for RaftKV.
  - The likely reason is etcd's higher number of indeterminate operations. This is inferred, not measured directly.
- **One etcd error message appears at node start in every E2 scenario.** It is `failed to update storage version ... missing term information`, 122 times in total, including in partition-only cells where no node restarts. It is etcd's own startup message and did not affect results.
- **RaftKV node logs** hold only INFO-level lines across all 180 trials, with no warnings, errors or panics.

## 6. RQ1 assessment

The evidence supports RQ1 for RaftKV with ReadIndex reads:

- 180/180 histories were linearizable across crash (including all nodes losing unsynced writes), pause, three partition shapes, clock skew and mixed faults.
- No reads were provably stale, and none of the 180 trials was inconclusive.
- Each scenario's upper bound on the violation rate is 0.16 (95%). Pooled, the bound is 0.02.

The method is validated. On etcd, under the same workload and faults, the harness reported no false positives: 0 Illegal and 0 stale reads. Every inconclusive history turned out linearizable when re-checked. The detector itself was shown to work in the limited E6 run, where 2 injected bugs were detected at rates of 1.00 and 0.88.

Limits on the claim:
- This is testing, not a proof. It covers 5 nodes, 34-second runs, one machine, two active keys per epoch, and these fault types and durations.
- Lease reads are outside RQ1 here. They are E5's subject.
- etcd's kill-all ran without LazyFS, so the E1-versus-E2 comparison is weaker for that scenario. There is no etcd counterpart for clock-skew.

## 7. Ready for E5?

**Yes.** E1 is clean on the read path E5 uses as its baseline, and the harness has a working positive control (limited E6) and a working negative control (E2).

Before or alongside E5, I recommend:

1. **Treat checker memory as a limit (essential).** E5's clock-partition cells will produce Illegal or Unknown histories. Those cost up to 13 GB here. Keep one trial per process, as these runs did.
2. **Optionally, improve the checker.** A per-key full check before reduction, plus the partial reduction above, resolved all 4 inconclusive trials offline. Adding these would be a harness change, so it waits for your approval. Without it, Unknown is reported as is.

No other test is essential for RQ1. A longer-run E1 (for example 120 s) would widen coverage, but it is not needed for the current claim.

## Reproduce

On Linux, as root, with `iproute2` and Go 1.26, at commit `80f06fc`:

```
export BIN=/tmp/raftkv-bin-e1 && make build && make bugtests
bash analysis/run.sh                         # E1 then E2, one trial per process
python3 analysis/aggregate.py raw > analysis/RESULTS.md
```

Offline re-check: copy `analysis/offline-recheck/zz_*_test.go` into `harness/`, then `go test -c -o recheck.test ./harness/` and run the commands in `offline-recheck/run-recheck*.sh`. These files are not part of the repository.

## Files

Everything is under `research/e1-e2-2026-10-15/`. No earlier result was touched.

- `raw/attempts.log`: one line per trial with exit code, wall time and peak RSS.
- `raw/E1/<scenario>/attempt-NNN/` and `raw/E2/<scenario>/attempt-NNN/`:
  - `command.txt` (exact command line), `console.log`;
  - `trial-001/` with `history.jsonl`, `nemesis.jsonl`, `n1..n5.log`, `result.json` (checker verdict);
  - `summary.json` (full configuration).
- The crashed trial also has `dmesg-tail.txt` and `offline-stale.txt`.
- `analysis/`: `RESULTS.md`, `aggregate.py`, `run.sh`, `stale_offline.py` and `offline-recheck/`.
- 3,742 raw files, 1.3 GB.
