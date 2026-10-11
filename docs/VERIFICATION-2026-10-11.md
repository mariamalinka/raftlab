# Verification after the follower-ack durability fix (R-1)

Date: 2026-10-11. Branch `fix/follower-ack-durability` (fix in `cfa86fa`, not merged to `main`).
Evidence: `results/verification-2026-10-11/`. Nothing under `results/` from earlier runs was changed.

Labels used below:
- **Verified**: shown by a test, a measurement or a checker result that is saved with this report.
- **Inferred**: follows from code reading or reasoning, not from a run.
- **Open**: not settled.

This report does not claim that the Raft implementation is correct. It covers five questions. Everything else in the implementation is exactly as tested as it was before: the unit suite, the harness schedules, and code review. Section 6 lists what is still untested.

## Summary

| # | Question | Answer | Status |
|---|---|---|---|
| 1 | Were the 3 after-fix bridge-churn Unknowns hidden violations? | No. All 3 are linearizable. Trial 5 passes a full check with more time. Trials 14 and 15 pass after a reduction that is sound for Ok: unobserved indeterminate writes are dropped. Each timeout came from one key. That key was blown up by indeterminate writes whose values were never read. | Verified |
| 2 | Persistence: §16.2 (un-awaited term), followers applying before fsync | No safety violation found or reproduced. Every vote and every successful append waits for the term and vote it carries. Only refusals, and one snapshot-covered success, can leave earlier. Applying before local fsync is harmless and recovers correctly. No Raft core change was made. | Verified (tests) + Inferred (argument) |
| 3 | Why ackbeforefsync detection fell from 3/3 to 1/3 | It was the harness schedule, not the fix. Commit `9667921` skips a fault that cannot finish before the workload ends, so a 20 s run fits 2 kill-all faults instead of 3. The third kill was the best detection window. Over 20 trials per arm, both builds lose acknowledged writes in 20/20 trials. With 3 kills, both detect it in 18/20. | Verified |
| 4 | Cost of waiting for stable storage before acking AppendEntries | No measurable throughput or p50 cost below saturation. Tail medians at 500–2000 ops/s are 4–71 % higher in all 12 cells, but no difference is significant at n = 5. Followers now wait on 25–92 % of requests without new entries, for about 1–3 ms each. Both raftkv builds collapse at 12,000 ops/s and etcd does not; the collapse predates the fix. | Verified (measured); cause of tail shift Inferred |
| 5 | Required changes | See section 5 | — |

## 1. The remaining Unknown results (bridge-churn, after the fix)

**Where:** `results/lost-write-2026-10-10/after-fix/bridge-churn-20`. Unknown trials: **5, 14, 15** (17 Ok, 0 Illegal). Each history has about 25k operations over 6 keys. The checker had a 60 s budget for the whole history.

**What was done** (outputs in `results/verification-2026-10-11/unknowns/`):
1. **Conservative stale-read scan** (`analysis/stale_reads.py`) on all keys of all 20 after-fix trials. It found 0 flagged reads.
   - The scan only flags a read that no non-superseded write, including indeterminate ones, could have produced.
   - So a 0 rules out only the simplest kind of violation. It does not prove linearizability.
2. **Per-key Porcupine, more time:**
   - Trial 5: all 6 keys **Ok**, with 600 s per key (147 s in total). `perkey-005.txt`.
   - Trials 14 and 15: with 60 s per key, exactly one key in each stays Unknown: trial 14 key 2 and trial 15 key 0. Every other key is Ok within 7 s. `scan-014-60.txt`, `scan-015-60.txt`.
   - Running the hard keys longer exhausted memory: one checker reached 9 GB and was OOM-killed.
3. **Prefix cuts on the hard keys** (`prefix-cuts.txt`):
   - Trial 14, key 2: the check is Ok up to 25 s (3,329 ops in 0.2 s) and Unknown from 27 s. Twelve writes and CASes became indeterminate at 25.09–25.14 s, just after a bridge crash.
   - Trial 15, key 0: Ok up to 10 s (1.6 s), Unknown from 12 s. Ten operations became indeterminate at 8.75–8.80 s.
4. **Reduction check** (`reduced-all.txt`; the scratch test source is saved as `scratch_hist_test.go.txt`).
   - Drop the indeterminate writes and CASes whose value is never read and never expected by any CAS on that key. Then re-check.
   - Trial 14 key 2 is **Ok** in 0.3 s (22 dropped). Trial 15 key 0 is **Ok** in 2.4 s (10 dropped). Every key of every after-fix Unknown trial is Ok.
   - Why this is sound for Ok: the register model is nondeterministic. An indeterminate op may always be linearized as having had no effect (`Step` returns `{s, ...}`). So any linearization of the reduced history extends to the original by placing each dropped op anywhere in its window with no effect.
   - The converse does not hold: Illegal on a reduced history is not a proof. That direction is not used for any after-fix result.
5. **Control on the before-fix Unknowns** (trials 5, 7, 12, 15, 16, 17, 18, 19):
   - The same reduction gives Ok for every key except **trial 7 key 3**, which is Illegal.
   - That is the same key, at the same time, as the 2 provably stale reads the conservative scan finds (26.11 s). It is the known R-1 loss.
   - So the reduction did not hide the one real violation present.

**Why Porcupine timed out** (Inferred, consistent with the cuts and the reduction):
- With unique values, each indeterminate write adds a value that nothing ever reads.
- Porcupine keeps such operations pending to the end of the history. Under the nondeterministic model it also tracks both "took effect" and "did not".
- The number of distinct (linearized set, state set) search nodes then grows with every pending unobserved write, and with the concurrency around it.
- Removing exactly those operations removes the blow-up.
- Neither the raw count of indeterminate ops nor the mean concurrency predicts which trials blow up. Trial 1 key 2 had 53 indeterminate ops and was Ok. See `shape.py`.

**Facts:**
- Verified: the 3 after-fix Unknowns are linearizable histories.
- Verified: none of the 3 is a correctness violation.
- Verified: each Unknown was caused by one key's search space, not by the history's size.

**Hypothesis:**
- The blow-up mechanism above. It is consistent with all observations, but I have not instrumented Porcupine itself.

**Open:**
- The first E6 rerun's Unknowns (ackbeforefsync, unique values) were not re-checked this way. There, the stale-read scan is the evidence.

**Required change:** add the reduction to the harness checker. Run it only after a plain check returns Unknown, and report the result as `Ok (reduced)` so that it is distinguishable from a plain `Ok`. This has not been done: it changes the checker, so it needs your decision.

## 2. Persistence audit

### 2.1 §16.2: `becomeFollower` does not wait for the new term (`internal/raft/raft.go:424-428`)

**Code paths** (Inferred, from reading the current branch):

| Response | Waits for stable term/vote? | Why |
|---|---|---|
| RequestVote, granted (`raft.go:773`) | Yes | `waitDurable(persistHardState())`. FIFO order means it also covers the term bump queued by `becomeFollower`. |
| RequestVote, refused (stale log, already voted, lower term) | No | Nothing is promised. The reply only reports a term. |
| AppendEntries, success (`raft.go:879`) | Yes, since `cfa86fa` | Waits for `enqSeq`, which covers the term bump and all queued entries. |
| AppendEntries, refused (gap or conflict) | No | Nothing is promised about the log. The leader still counts the reply as a contact for ReadIndex and lease (`recordAck`). See below. |
| AppendEntries, success when all entries are inside the snapshot (`raft.go:811`) | **No** | It reports a match only up to `snapIndex`. That is at most `lastApplied`, so at most the follower's commit, so these entries are already committed. The term carried may not be stored. |
| InstallSnapshot | Yes | Waits for its own persist op, which is behind the term bump. |
| Candidate's own vote (`raft.go:454`) | Yes | Waits before sending RequestVote. |

**Why an un-stored term is harmless** (Inferred):
- A crash rolls a node back to its last stored term. Every vote and every successful log ack is made only after the term it is made in is stored.
- So a node never has a promise in a term later than its stored term.
- After the rollback, the node may accept a leader of an earlier term. That is indistinguishable from a delayed message, which Raft already tolerates.
- It cannot vote twice in a term, because the vote is stored with the term.
- **Lease and ReadIndex.** Here the node did promise something in the new term without storing it: a refusal or a snapshot-covered success still counts as an ack for the leader's lease.
  - What protects the lease is the restart embargo (`inLease`, `raft.go:724`). A restarted node refuses higher-term votes for one election timeout from boot.
  - The lease is shorter than that and started before the ack, so it has expired before the node can vote.
  - This is the same mechanism that protects leases when the term *was* stored. It relies on the clock-rate assumption the lease already makes.

**Tests** (new file `internal/raft/persist_audit_test.go`; writes are held by a gated storage, and the clock only moves when the test moves it):

| Test | What it checks | Result |
|---|---|---|
| `TestPromisesWaitForDurableTerm/vote` | A granted vote never returns before the term and vote are stored | Pass |
| `TestPromisesWaitForDurableTerm/append` | A successful append in a new term never returns before that term is stored | Pass |
| `TestRefusalsMayCarryUnstoredTerm` | Refusals in terms 6 and 7 leave while the stored term is 5. After a crash the node is back in term 5 and still refuses a second vote in term 5. | Pass (documents the behaviour) |
| `TestSnapshotCoveredAppendDoesNotWait` | The snapshot-covered success leaves immediately with term 7 unstored, and match ≤ commit | Pass (documents the behaviour; skips if the path changes) |

- **Mutation check** (`results/verification-2026-10-11/persistence-audit/mutation-tests.txt`): the tests were run on a scratch copy with each wait removed.
  - Removing the vote's `waitDurable` makes `/vote` fail.
  - Removing the AppendEntries durability wait, which is the pre-fix behaviour, makes `/append` fail.
  - So the tests can see the property they check.

### 2.2 Followers apply committed entries before their own log is durable

**Code** (Inferred): `HandleAppendEntries` sets `commit = min(req.Commit, newLast)` at `raft.go:865`, before the durability wait. `HandleInstallSnapshot` sets `commit` and the pending snapshot before its wait. The applier can therefore deliver an entry that is not yet in this follower's stable log.

**Why it is safe** (Inferred):
- An index ≤ `req.Commit` was committed by the leader, which means it is stored on a majority.
- The follower's entry at that index matches the leader's: the `prev` check passed and the log was reconciled up to `newLast` in this same call.
- If the follower crashes before writing it:
  - The follower reloads a shorter log.
  - Its state machine is in memory only and is rebuilt from the snapshot and the log.
  - The leader then sends the same entries again.
- A local `Snapshot()` can only cover applied, hence committed, entries, and it is persisted in queue order.
- Thesis §10.2.1 allows the same decoupling.

**Test** `TestFollowerAppliesBeforeStoringThenRecovers`:
- The test asserts that follower F applies X while its storage is held and X is not stored. Then F crashes, restarts, and applies the same X at the same index. No other node applies anything else there.
- Result: passes 20/20.

### 2.3 Other observations from the audit (not defects)

- `durableLast` is the `lastIndex()` taken when the op was enqueued. It is exact because every in-memory append is enqueued in the same critical section: in `Propose`, `becomeLeader`, `HandleAppendEntries` and `Snapshot`. A future code path that appends without enqueueing would make `selfMatch` overstate durability. (Inferred.)
- AppendEntries responses from an earlier term are ignored (`r.term != req.Term`). Match only increases. Pipelined responses that arrive out of order cannot lower it. (Inferred.)

**Facts:**
- Verified: no promise leaves before its term or vote is stored, on the paths tested.
- Verified: applying before local durability recovers correctly in the tested scenario.

**Hypotheses:** none.

**Open:**
- The lease argument depends on the restart embargo and bounded clock rate. It is not tested under crash plus clock drift together.
- The snapshot-covered success path is only argued safe, not exercised end to end.

**Required changes:** none to the core. Optionally, make the snapshot-covered success path wait like the others, purely for uniformity. That is not justified by any failing test, so it was not done.

Full suite on the branch, including the new tests:
- `go vet` clean.
- `go test -race ./...` passes.
- `make bugtests` catches all 4.
- See `results/verification-2026-10-11/unit-suites.txt`.

## 3. ackbeforefsync detection: 3/3 before, 1/3 after

**Cause** (Verified):
- The 3/3 run (`results/rerun-2026-10-09/E6/ackbeforefsync/kill-all-rotate15-unique`) used the harness from before `9667921` and had **3** kill-all faults per trial.
- The 1/3 run (`results/lost-write-2026-10-10/after-fix/E6-ackbeforefsync-still-detectable`) used the current harness. Since `9667921` (`harness/nemesis.go:283`) it skips a fault that cannot finish before the workload ends. With `-time 20s`, the third kill at 19 s does not fit, so the run had **2** kills (see each trial's `nemesis.jsonl`).
- The second kill heals at 16 s, after the key rotation at 15 s, so its lost writes are never read.
- So the after-fix run had one detection window per trial instead of two.
- The methodology change was disclosed in `docs/EXPERIMENTS.md` when it was made. Its effect on this comparison was not noticed until now.

**Repeat with 20 trials per arm** (`results/verification-2026-10-11/ackbeforefsync/SUMMARY.txt`):
- Settings: same harness and settings for both arms (kill-all, `-lazyfs`, unique values, `-key-rotate 15s`, 25 clients), builds alternating in blocks of 5.
- "Proven loss": the number of writes and CASes acknowledged before the kill exceeds the largest `last_index` any node reloaded after it. That is a lower bound, because no-ops and retries only lengthen the log, and the count is cumulative.
- "Stale": provably stale reads after the heal.

| Arm | Kills | Proven write loss | Reads of pre-kill keys after heal | Detected (stale reads) | Porcupine Ok / Unknown / Illegal |
|---|---|---|---|---|---|
| old build, 20 s | 2 | 20/20 | kill 1 only, 20/20 | 15/20 [0.53, 0.89] | 5 / 15 / 0 |
| new build, 20 s | 2 | 20/20 | kill 1 only, 20/20 | 11/20 [0.34, 0.74] | 6 / 14 / 0 |
| old build, 25 s | 3 | 20/20 | kills 1 and 3, 20/20 | 18/20 [0.70, 0.97] | 2 / 18 / 0 |
| new build, 25 s | 3 | 20/20 | kills 1 and 3, 20/20 | 18/20 [0.70, 0.97] | 2 / 18 / 0 |

**What the numbers show:**
- Old vs new, 20 s: Fisher exact p = 0.32. Old vs new, 25 s: p = 1.00.
- Median deficits after the first kill are 14 vs 15 (20 s) and 27 vs 12.5 (25 s). After the third kill they are 536 vs 570.
- The fix did not change how much is lost or whether it is read.
- In every arm, Porcupine never returned Illegal. Every detection came from the conservative stale-read scan.
- Every trial with stale reads is Unknown. The "Ok" trials are trials whose losses were not observed by any read, so they are not false negatives of the checker.

**Facts:**
- Verified: the injected loss still happens in 20/20 trials with the fixed build.
- Verified: affected keys are read after recovery for kills 1 and 3, never for kill 2.
- Verified: with the original 3-kill schedule, detection is 18/20 for both builds.

**Hypotheses:**
- The earlier 3/3 vs 1/3 gap is fully explained by the missing third kill plus sampling noise at n = 3. This is consistent with all the data above.

**Open:**
- Porcupine alone detects ackbeforefsync in 0 of 80 trials.

**Required changes:**
- Make every E6/E1 config that needs N faults set `-time` so that N faults fit, or log the number of faults run in `summary.json` and report it.
- The reduction from section 1 might let Porcupine conclude on these histories. Untested.

## 4. Performance

**Setup** (`results/verification-2026-10-11/bench/`, script `bench/run.sh`):
- Systems: unfixed raftkv `9667921` (= `main`), fixed `cfa86fa`, and etcd 3.7.2. All were built fresh from those commits.
- Configuration, identical for all three: 5 nodes on the same 4-core VM, election timeout 1 s, heartbeat 100 ms, fsync on, default batching and pipelining, ReadIndex reads for raftkv and linearizable reads for etcd.
- Workload: 10k keys, 128 B values. Open-loop offered load of 500–12,000 ops/s; each point is 20 s after a 5 s warmup.
- Read fractions: 0.5 (the E7 mix) and 0 (write-only).
- Five repetitions per point, run as separate invocations interleaved across the three systems.
- `SUMMARY.md` has every point with medians and min–max.

**Fixed vs unfixed, below saturation** (median of 5; ratio new/old; exact permutation p):

| Read frac | ops/s | p50 | p99 | p99.9 |
|---|---|---|---|---|
| 0.5 | 500 | 1.79 → 1.85 ms (×1.04, p 0.78) | 5.30 → 6.29 (×1.19, p 0.98) | 18.1 → 22.4 (×1.24, p 0.94) |
| 0.5 | 1000 | 1.97 → 2.12 (×1.08, p 0.79) | 8.70 → 11.38 (×1.31, p 0.19) | 18.4 → 31.4 (×1.71, p 0.048) |
| 0.5 | 2000 | 3.34 → 3.69 (×1.11, p 0.91) | 27.2 → 34.4 (×1.26, p 0.16) | 53.4 → 56.3 (×1.05, p 0.25) |
| 0.5 | 4000 | 6.69 → 7.09 (×1.06) | 55.3 → 53.6 (×0.97) | 75.9 → 77.6 (×1.02) |
| 0.5 | 8000 | 17.9 → 18.0 (×1.01) | 88.6 → 86.5 (×0.98) | 107 → 121 (×1.13) |
| 0 | 500 | 2.19 → 2.21 (×1.01) | 10.5 → 11.9 (×1.14, p 0.32) | 23.9 → 29.9 (×1.25, p 0.89) |
| 0 | 1000 | 2.93 → 2.91 (×0.99) | 15.0 → 22.2 (×1.48, p 0.11) | 29.0 → 35.7 (×1.23, p 0.07) |
| 0 | 2000 | 4.48 → 4.42 (×0.99) | 36.0 → 37.3 (×1.04) | 45.6 → 52.7 (×1.15) |
| 0 | 4000 | 7.25 → 7.57 (×1.04) | 53.2 → 58.6 (×1.10) | 67.3 → 75.9 (×1.13) |
| 0 | 8000 | 32.6 → 27.8 (×0.85, p 0.11) | 110 → 101 (×0.92) | 145 → 128 (×0.88) |

- **Throughput:** every point up to 8,000 ops/s is served in full by both builds, with 0 errors.
- **Latency:** p50 is unchanged. Tail medians are higher at 500–2000 ops/s in all 12 cells (by 4–71 %). Only one cell has p < 0.05 (out of 30 comparisons, so it is not significant after any correction), and min–max ranges overlap everywhere.
- **Verified:** no detectable throughput or median cost.
- **Not established:** a tail cost at low load. The direction is consistent, but it needs more repetitions to confirm or rule out.

**Saturation, 12,000 ops/s:**
- Both raftkv builds collapse. Achieved throughput is 128–4,637 ops/s (medians), with hundreds of thousands of errors and drops, and the variance is huge.
- etcd serves the full 12,000 (p99 204 ms in the mix, 312 ms write-only).
- The unfixed build collapses too, so the fix did not cause this.
- It is a pre-existing overload problem in raftkv: no admission control or backpressure (Inferred, not investigated).

**raftkv vs etcd below saturation:**
- raftkv p50 is about half of etcd's at every load. For example, at 4,000 ops/s in the mix: 7.1 vs 13.0 ms.
- Tails:
  - In the 0.5 mix, etcd has lower p99 at 2,000–4,000 ops/s (21 vs 34 ms and 38 vs 54 ms).
  - Write-only, raftkv has the lower p99 from 2,000 ops/s up.
  - At 500–1,000 ops/s, write-only, etcd's tails are lower.
- These are single-VM numbers: all 5 nodes share 4 cores and one disk. p50 grows almost linearly with load for every system, which points to CPU contention, not consensus cost. Don't read them as a real deployment comparison.

**Direct measurement of the added waits** (`results/verification-2026-10-11/bench-waits/`):
- Method: a scratch build of `cfa86fa` counts and times every follower durability wait in `HandleAppendEntries`. The build is not in the repository; the patch is saved as `instr_raft.go.diff` and `instr_waits.go.txt`.

| Read frac | ops/s | Requests with entries: waited / mean | Requests without new entries: waited / mean | ≥ 6.4 ms |
|---|---|---|---|---|
| 0 | 1000 | 100 % / 1.05 ms | 86 % / 1.60 ms | 4.0 % |
| 0 | 4000 | 100 % / 1.64 ms | 92 % / 1.74 ms | 3.3 % |
| 0 | 8000 | 100 % / 2.73 ms | 86 % / 3.03 ms | 10.4 % |
| 0.5 | 1000 | 100 % / 0.98 ms | 25 % / 1.03 ms | 2.2 % |
| 0.5 | 4000 | 100 % / 1.41 ms | 67 % / 1.40 ms | 1.9 % |
| 0.5 | 8000 | 100 % / 1.93 ms | 57 % / 2.05 ms | 5.2 % |

**What the waits show:**
- Requests that carry entries always waited, before the fix too. The new waits are on requests without new entries: heartbeats, ReadIndex probes and retransmissions.
- Under writes, most of these now wait for the group fsync already in progress, about 1–3 ms each.
- In the mixed workload these are mostly ReadIndex probes, so a read's confirmation round can take one fsync longer (Inferred). That is the most likely source of the higher tails at low load. It is not proven: the benchmark does not separate read latency from write latency.

**Possible optimisation** (not done; it changes the core and no test demands it):
- For a request without new entries, the follower could answer at once with `MatchIndex = min(newLast, durableLast)` instead of blocking. That claims only what is stored, which is what Figure 2 requires.
- This keeps ReadIndex probes and lease acks fast.
- It would need its own regression test, so that the R-1 tests still fail when the claim exceeds storage.

## 5. Required changes, by priority

1. **Merge decision on `fix/follower-ack-durability`** (needs your approval).
   - Nothing found here argues against it. The fix is needed for safety (R-1), shows no measurable throughput or median cost, and passes every test listed above.
2. **Fault count in results.**
   - Record the number of faults actually run in `summary.json` and print it in `analysis/report.py`.
   - Set `-time` so that the intended number of faults fits, for example 25 s for 3 × 5 s faults with 3 s gaps.
   - Without this, schedules silently change between runs. That is what produced the 3/3 → 1/3 drop.
3. **Checker: sound reduction for Unknown.**
   - When a key is Unknown, re-check it with unobserved indeterminate writes and CASes removed, and report `Ok (reduced)`.
   - Ok is sound (section 1). Illegal from the reduced history must still be confirmed, for example by the stale-read scan.
4. **Report stale-read scan results next to Porcupine in E6.**
   - Porcupine returned Illegal in 0 of 80 ackbeforefsync trials. All 62 trials with a detection got it from the scan.
5. **Overload behaviour.**
   - raftkv collapses at 12,000 ops/s on this VM while etcd does not. This affects the plan's RQ2 latency/throughput comparison.
   - Needs its own investigation: backpressure, proposal queue limits, client timeouts.
6. **Optional:** the non-blocking `MatchIndex = min(newLast, durableLast)` reply from section 4, and making the snapshot-covered success path wait for the term (section 2.3). Neither is required by any failing test.
7. **More repetitions** at 500–2000 ops/s, with reads and writes recorded separately, to settle whether the fix costs tail latency.

## 6. What this does not show

- That the implementation is correct. The tests check specific properties on specific paths. The harness explores a few fault schedules on one machine, and the checker is incomplete (Unknown) on the histories most likely to contain anomalies.
- Untested here:
  - Membership changes (not implemented).
  - Disk faults other than lost unsynced writes (torn or corrupted records are only truncated at load).
  - Clock-rate violations combined with crashes.
  - Long runs, and more than 5 nodes.
- The persistence argument for refusals and the snapshot-covered path (section 2) is reasoning plus tests of the local behaviour. No end-to-end scenario was built for it.
- The performance numbers come from 5 nodes sharing one 4-core VM and one disk. They compare builds fairly with each other, but they are not the deployment numbers the research plan asks for.

## Artifacts

| Path | Contents |
|---|---|
| `results/verification-2026-10-11/unknowns/` | Per-key, prefix and reduced Porcupine checks; shape script; scratch checker test source |
| `results/verification-2026-10-11/persistence-audit/` | Audit test output; mutation test output |
| `results/verification-2026-10-11/ackbeforefsync/` | 80 trials (4 arms × 4 blocks × 5); `SUMMARY.txt`; `ackloss.py`; `run.sh`; per-trial JSON |
| `results/verification-2026-10-11/bench/` | 30 sweeps (3 systems × 2 mixes × 5 reps); `SUMMARY.md`; HDR percentiles; `run.sh` |
| `results/verification-2026-10-11/bench-waits/` | Instrumented wait counters per load point; instrumentation patch |
| `results/verification-2026-10-11/unit-suites.txt` | vet, race suite, bug tests on the branch |
| `internal/raft/persist_audit_test.go` | The 4 audit tests (5 test cases) |
