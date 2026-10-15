# RaftLab: G1 baseline and E6 readiness (2026-10-14)

## 1. Repository state

- GitHub `main` is at `3055bd5ae643ae426ecdef097e05a0b8203ceeb7`. Your Windows checkout's `main` and `origin/main` both point there. I verified this from a clone of the device's `.git`, because this session cannot reach the private repository directly.
- Working tree: clean (`git status --porcelain` is empty on a fresh checkout of `3055bd5`). Uncommitted edits in the Windows working tree itself were not checked.
- **R-1 fix present:** `00d9651` "raft: follower acknowledges AppendEntries only after its log is stored". The follower waits for every queued write before it reports `MatchIndex` (`internal/raft/raft.go` 872 to 886). Its regression tests, `TestFollowerAckRequiresStableStorage` and `TestCommittedEntrySurvivesFollowerCrash`, pass, as do the 4 persistence-audit tests.
- **Harness and checker changes present:** `de293fa` and `31aea1c` add Ok (reduced) after Unknown, the separate stale-read scan, and planned / executed / completed fault counts.
- **Test infrastructure:** 36 Raft tests (cluster, durability, persistence audit, 3 white-box bug tests), the kv `TestBugNoDedup`, and 8 harness checker tests. `make bugtests` covered 4 of the 7 bugs.

## 2. G1: Raft unit suite, 500 runs per configuration

| Configuration | Flag | Runs | Passed | Failed | Race warnings | Mean run | Wall clock |
|---|---|---|---|---|---|---|---|
| Default | (none) | 500 | 500 | 0 | 0 | 156.7 s | 2.74 h |
| Pipelining | `-pipeline` | 500 | 500 | 0 | 0 | 154.1 s | 2.70 h |
| Batching off | `-batch=1` | 500 | 500 | 0 | 0 | 175.0 s | 3.06 h |

**G1 passes:** 1,500 of 1,500 runs, with no flaky or intermittent failures, no retries and no data races.

- Commit `3055bd5`, Go 1.26.0 linux/amd64, 4 vCPU.
- Total wall clock 3.06 h, from 19:29:48 to 22:33:33 UTC. The runs add up to 67.5 hours of test time.
- Build: `go test -c -race -o /tmp/rk/raft.test ./internal/raft/`
- Run: `/tmp/rk/raft.test -test.count=1 -test.v [-pipeline | -batch=1]`. This is the same binary and the same three modes as `scripts/g1.sh`.
- **One deviation from `scripts/g1.sh`:** runs were 8-way parallel per mode, so 24 processes at once. A single run uses about 8% of one core and mostly waits on timers, so the extra load can only make timing-sensitive tests harder, not easier. During the first ~30 minutes, short test jobs also ran alongside.
- Raw per-run data: `G1/runs-*.csv` (exit code, duration, failed tests, race warnings, start time). The scripts are in `G1/`. No failure logs exist, because no run failed.

## 3. E6 readiness per bug

Details are in `E6-READINESS-AUDIT.md`. The smoke data (3 trials each, proposed settings) is in `readiness-smoke/`.

| Bug | Deterministic test | Smoke (target fault) | Ready? |
|---|---|---|---|
| ackbeforefsync | **new**, 100/100 caught | kill-all + LazyFS: stale reads 3/3, but Porcupine **Unknown 3/3** | Ready (B2 decided: combined rate, `patches/0003`) |
| localread | **new**, 100/100 caught | partition-leader: Illegal 1, Unknown 1, Ok 1; stale reads 2/3 | Yes |
| leasenodrift | **new**, 100/100 caught | clock-partition-mild: Illegal 1/3; correct lease control 0/3 | Yes (mixed cell confounded, B5) |
| nodedup | existing | pause-leader: Ok 3/3 | Yes, low rate expected |
| notruncate | existing | partition-halves: Ok 3/3 | Expected blind spot (B4) |
| figure8 | existing (white-box) | mixed: Ok 3/3 | Expected blind spot (B4) |
| votenopersist | existing | kill-majority: Ok 3/3, 0 two-leader terms | Measured as is (B3 decided); expected 0 |

Cross-cutting checks:

- Every node start logged the requested bug, in all 33 smoke trials.
- All 99 faults ran their full 5 s (5.00 to 5.10 s), and executed = planned = 3 in every trial.
- Porcupine, Ok (reduced) and the stale-read scan are reported separately.
- Controls: 9 correct-build trials, 0 Illegal, 0 stale reads.
- **Unique values and key rotation are not set by the current E6 script.** `patches/0002` fixes this.

## 4. Blockers

1. **B1, E6 script settings:** fixed by `patches/0002`. You only need to apply it.
2. **B2, detection metric (decided 2026-10-15: both rates):** ackbeforefsync is invisible to the Porcupine-only rate because every trial is Unknown. My recommendation is to make "Illegal or provably stale read" the primary detection rate and keep Porcupine-only as the secondary rate.
3. **B3, votenopersist (decided 2026-10-15: measure as is):** the restart embargo makes a same-term double vote practically unreachable with the current faults, and bridge-churn stalls elections outright. Either measure it as an expected 0, or add a dedicated nemesis (a harness change).
4. **B4, no bug-hit counters:** a 0 cell for figure8 or notruncate cannot show whether the path fired. Adding counters would touch `raft.go` bug branches, which needs your approval because you asked for no Raft-core changes.
5. **B5, leasenodrift × mixed:** the mixed pool includes the x0.25 clock fault that breaks correct leases too. A lease control is added; dropping mixed for this bug is cleaner.

## 5. Recommended next actions

1. Apply the two patches in Git Bash and push (commands below).
2. B2 and B3 are decided. B4 and B5 stay as documented caveats.
3. Run the previously Unknown cells first: `BUGS="ackbeforefsync localread" TRIALS=20`. This should take about 2 to 4 h.
4. Run the full matrix, 74 cells × 20 trials ≈ 28 h (plan for 20 to 40 h). Then check `AUDIT.md` against the acceptance criteria in `E6-EXECUTION-PLAN.md`.
5. Rerun E1 and the E2 etcd control with the same settings, so E6 controls and E1 share one baseline.

## 6. Patches

| Patch | Contents |
|---|---|
| `0001` | `TestBugAckBeforeFsync`, `TestBugLeaseNoDrift`, `TestBugLocalRead`. `make bugtests` covers all 7 and only counts a real test failure (a build error used to count as "caught"). Adds the read-only `analysis/e6_audit.py`. |
| `0003` | `analysis/report.py`: a "Combined detection" column, counting Illegal or a provably stale read over all trials without errors, with Unknown kept in the denominator. The Porcupine-only column is unchanged. |
| `0002` | E6 block of `scripts/experiment.sh`: unique values, 12 s key epochs, a 34 s run, correct-build controls, and `AUDIT.md`. |

- Both patches are authored and committed as `mariamalinka <marysiamalinka2@gmail.com>`, with no co-author lines.
- They apply cleanly on `3055bd5`, and on the result `go vet`, `go test -race ./...` and `make bugtests` (7/7 caught) all pass.
- No Raft core, KV, harness or checker code is changed.

SHA-256:
```
c4ab240cab22fe171ec7027c227196ca1ca36dc77e7ba1be9911c4d1764ac58a  0001-Deterministic-tests-for-ackbeforefsync-leasenodrift-.patch
7e1431502ea2c15773df96d5a08316de0b4e29eb712dea0ff13fa007b84efd6d  0002-E6-unique-values-12-s-key-epochs-a-34-s-run-that-fit.patch
0f7c82c6f35fa28490e377a285d8ce1bfdfef933724b155cde01beac62446863  0003-report-combined-detection-rate-Illegal-or-provably-st.patch
```

## 7. Reproduce

Apply (Git Bash, Windows):
```
cd ~/raftlab/raftkv
git checkout main && git pull
git am ../g1-e6-baseline/patches/*.patch
git log --oneline -3
git push
```

G1 (Linux, from the repository root):
```
go test -c -race -o /tmp/raft.test ./internal/raft/
cd internal/raft
for i in $(seq 500); do /tmp/raft.test -test.count=1 || echo "default run $i FAILED"; done
for i in $(seq 500); do /tmp/raft.test -test.count=1 -pipeline || echo "pipeline run $i FAILED"; done
for i in $(seq 500); do /tmp/raft.test -test.count=1 -batch=1 || echo "batch1 run $i FAILED"; done
# or the parallel driver used here: bash G1/g1-parallel.sh <outdir>   (RUNS=500 P=8)
```

Deterministic bug tests: `make bugtests`

Readiness smoke (root, `iproute2`): `make build && bash readiness-smoke/smoke.sh`, then `python3 analysis/e6_audit.py <outdir>`

## Files

All under `research/g1-e6-baseline/`. Every earlier artifact is untouched.

- `REPORT.md`: this file
- `E6-READINESS-AUDIT.md`
- `E6-EXECUTION-PLAN.md`
- `G1/`: results, CSVs, scripts and environment
- `readiness-smoke/`: 33 trials, `AUDIT.md`, `REPORT.md`
- `patches/`
