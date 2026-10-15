# E6 readiness audit (2026-10-14)

Commit audited: GitHub `main` at `3055bd5` (clean checkout, Maria's device copy tracks `origin/main`).
Evidence: code reading, the deterministic tests in `patches/0001`, and a readiness smoke of 11 cells × 3 trials
with the proposed E6 settings (`readiness-smoke/`, audit table `readiness-smoke/AUDIT.md`).

Smoke settings: 5 nodes, 25 clients, `-time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values`.

## Per-bug status

| Bug | 1 Flag works | 2 Path can fire | 3 Deterministic test | 4 Fault creates precondition | 5 Workload can observe | 6 Checker result in smoke | Status |
|---|---|---|---|---|---|---|---|
| ackbeforefsync | Yes, 20/20 node starts logged it | Yes | **New** `TestBugAckBeforeFsync`, 100/100 caught | Yes for kill-all + `-lazyfs`; **no** for kill-majority (0/3 stale reads) | Yes, stale-read scan 3/3 (5, 9, 7 stale reads); control 0/3 | **Unknown 3/3**: Porcupine never decides, so the primary detection rate is 0/0 | Blocked on the detection metric (B2) |
| figure8 | Yes | Only white-box: the new leader's no-op travels with the old entries | `TestBugFigure8` (existing) | **Not shown.** No in-run evidence the commit-by-counting branch ever fired | Would need a Figure 8 overwrite sequence; none seen | Ok 3/3 under mixed | Expected blind spot (B4) |
| leasenodrift | Yes | Yes, needs `-read-mode lease` | **New** `TestBugLeaseNoDrift`, 100/100 caught | Yes, clock-partition-mild (x0.92, inside the 10% bound) | Yes | **Illegal 1/3** (at 28.5 s, 1 stale read); correct lease control 0/3 | Ready, except the mixed cell is confounded (B5) |
| localread | Yes | Yes | **New** `TestBugLocalRead`, 100/100 caught | Yes, partition-leader | Yes, stale reads in 2/3 | **Illegal 1/3**, Unknown 1/3, Ok 1/3; control 0/3 | Ready |
| nodedup | Yes | Yes (client retries keep the same sequence number) | `TestBugNoDedup` (existing) | Plausible under pause-leader (late commit of a retried write), not observed | Only with unique values and a read on both sides of another client's write | Ok 3/3 | Ready to measure; expect a low rate |
| notruncate | Yes | Yes. Conflict truncations happen (0 to 2 per trial) | `TestBugNoTruncate` (existing) | Conflicts yes; a stale suffix *longer than the new leader's log* (needed for a visible effect) not shown | Rarely: only if the divergent node later leads | Ok 3/3 | Expected blind spot (B4) |
| votenopersist | Yes | Yes (same-term requests skip the restart embargo) | `TestBugVoteNoPersist` (existing) | **No.** 0 terms with two leaders in 3 kill-majority trials (this smoke) and 3 bridge-churn trials (2026-10-10). Reason below | No two-leader term, so nothing to observe | Ok 3/3 | Blocked on a fault schedule (B3) |

"Flag works" is checked per trial: `analysis/e6_audit.py` confirms every `raft started` line in every node log lists the requested bug (all 33 smoke trials).

## Cross-cutting checks

| Check | Before (current `scripts/experiment.sh E6`) | With `patches/0002` | Evidence |
|---|---|---|---|
| Unique write values | **Off** (harness default) | On (`-unique-values`) | Without them ackbeforefsync gave Ok 3/3 (rerun-2026-10-09); with them, stale-read scan 3/3 |
| Key rotation | **5 s**, the same as a fault, so reads after a heal use new keys | 12 s epochs: each fault has its own keys plus 4 s of healthy reads after the heal | Faults at 3, 15, 27 s; epochs 0, 12, 24 s |
| Every planned fault runs its full duration | 30 s run fits 3 faults with 6 s margin | 34 s run fits 3 faults with 7 s margin | Smoke: 99/99 faults ran 5.00 to 5.10 s; none cut short, no fault errors |
| Executed fault count recorded | Yes (`faults_planned`, `faults`, `faults_completed`) | Same, plus per-fault real duration in `AUDIT.md` | Smoke: 3/3/3 in every trial |
| Porcupine, reduced check, stale-read scan reported separately | Yes: `valid`, `valid_reduced`, `unknown`, `illegal`, `stale_read_trials` | Same | `readiness-smoke/REPORT.md` |
| Correct-build controls under E6 settings | None in E6 | Correct build × every E6 fault, on readindex and lease | Smoke controls: 0 Illegal, 0 stale reads in 9 trials |

## Blockers

- **B1. E6 script settings (fixed by `patches/0002`).** Unique values off and 5 s key epochs. The patch also adds correct-build controls and writes `AUDIT.md` after the run.
- **B2. Detection metric for Unknown trials (decided 2026-10-15: both rates, implemented in `patches/0003`).** The detection rate counts only Porcupine Illegal over Ok + Ok (reduced) + Illegal. ackbeforefsync was Unknown in 3/3 trials, so its rate is 0/0 even though every trial had provably stale reads. Recommended: report two rates side by side. The primary rate counts a trial as detected if Porcupine says Illegal **or** the stale-read scan finds a provably stale read, over all completed trials. The secondary rate is Porcupine alone. The scan had no false positives: 0 in all 9 control trials. The only stale reads it ever found on a correct build were the real R-1 lost write, which is now fixed.
- **B3. votenopersist has no working fault (decided 2026-10-15: option (a), measure as is).** The double vote needs a voter to crash between two same-term vote requests and come back before the second arrives. Candidates send each request once per term. A restarted node also refuses any higher-term request for one election timeout after boot. In bridge-churn the bridge restarts every 100 to 400 ms, so it never leaves that embargo and never adopts a new term. Elections stall (4 leader terms per trial), and no same-term double vote can happen. Options:
  - (a) Measure it anyway. Expect 0, and cite `TestBugVoteNoPersist` as reachability evidence.
  - (b) Add a dedicated nemesis that restarts a voter less often than once per election timeout, during forced split votes. This is a harness change.
- **B4. No in-run proof that figure8 / notruncate branches fire.** Without hit counters, a 0 cell cannot tell "path never fired" from "fired, not observed". Counters would touch the bug branches in `internal/raft/raft.go`. Those branches are compiled out of release builds, but this still modifies the Raft core, so it needs your approval. Otherwise report these cells as expected zeros, backed by their deterministic tests.
- **B5. leasenodrift × mixed is confounded.** The mixed pool includes clock-partition at x0.25, which breaks the correct lease build too (E5 finding). `patches/0002` adds the correct-lease control under mixed, so the cell can be read against it. A cleaner alternative is to drop mixed for this bug.

## Not changed

No Raft core, KV, harness, nemesis or checker code was changed. The two patches touch only test files, `Makefile` (`bugtests`), `scripts/experiment.sh` (E6 block) and a new read-only `analysis/e6_audit.py`.
