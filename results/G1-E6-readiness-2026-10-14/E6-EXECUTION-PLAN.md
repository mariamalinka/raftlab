# E6 execution plan (RQ3 / H2)

Status: **not started.** B2 and B3 were decided on 2026-10-15 (both rates; votenopersist measured as is). Run it once `patches/0001` to `0003` are on `main`.

## Matrix

20 trials per cell, fresh 5-node cluster per trial, 25 clients.

| Group | Bugs / build | Faults | Extra flags | Cells | Trials |
|---|---|---|---|---|---|
| Bug cells | figure8, localread, leasenodrift, votenopersist, notruncate, nodedup, ackbeforefsync | kill-leader, kill-majority, pause-leader, partition-leader, partition-halves, partition-bridge, clock-partition-mild, mixed | leasenodrift: `-read-mode lease`; ackbeforefsync: `-lazyfs` | 56 | 1,120 |
| ackbeforefsync extra | ackbeforefsync | kill-all | `-lazyfs` | 1 | 20 |
| Control, readindex | correct build | the 8 faults above | none | 8 | 160 |
| Control, lease | correct build | the 8 faults above | `-read-mode lease` | 8 | 160 |
| Control, LazyFS | correct build | kill-all | `-lazyfs` | 1 | 20 |
| **Total** | | | | **74** | **1,480** |

Per-trial settings (from `patches/0002`): `-time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values -clients 25`. Faults start at 3, 15 and 27 s. Every cell plans 3 faults.

votenopersist is measured with the existing faults (decision B3, 2026-10-15). No new nemesis is added.

## Commands

On a Linux machine as root, from the repository root, with Go 1.26 and `iproute2`:

```
git checkout main && git log -1 --format=%H        # record the commit
apt-get install -y iproute2                          # if `ip` is missing
export BIN=/tmp/raftkv-bin OUT=results/E6-$(date -u +%F)
make build
make bugtests                                        # all 7 must say "caught"
TRIALS=20 bash scripts/experiment.sh E6              # whole matrix, controls included
python3 analysis/report.py $OUT > $OUT/REPORT.md
# $OUT/E6/AUDIT.md is written by experiment.sh at the end
```

To resume or split the run, narrow it with `BUGS`, `FAULTS` and `CONTROLS`, for example:

```
BUGS="ackbeforefsync localread" CONTROLS=0 TRIALS=20 bash scripts/experiment.sh E6   # the previously Unknown cells first
BUGS=" " TRIALS=20 bash scripts/experiment.sh E6                                       # controls only
```

Note that `BUGS=" "` (a space) skips every bug cell. An empty `BUGS` falls back to all seven.

## Expected duration

The readiness smoke measured 33 trials with these settings: median 46 s, mean 69 s, range 41 to 162 s. Unknown and Illegal trials take longer, because the reduced check and the HTML visualisation add time. 1,480 trials × 69 s ≈ **28 hours**. Plan for **20 to 40 hours**, run serially, since the network namespaces allow one cluster per machine. Disk use: about 4 MB per trial in the smoke, so roughly **6 GB** in total (node logs dominate).

## Metrics per cell

- Outcomes counted apart: Ok, Ok (reduced), Unknown, Illegal, errors.
- Porcupine detection rate: Illegal / (Ok + Ok (reduced) + Illegal), with a Wilson 95% CI, and the median time to detection.
- Stale-read scan: trials with at least one provably stale read, and the total count of stale reads.
- **Primary metric (decision B2):** the combined detection rate, (Illegal or stale > 0) / trials without a harness error, with Unknown kept in the denominator and a Wilson 95% CI. Shown as the "Combined detection" column (`patches/0003`). The Porcupine-only rate is reported next to it.
- Unknown share: Unknown / trials.
- Faults: planned, executed and completed per trial, plus the shortest real fault duration (from `AUDIT.md`).
- Bug evidence from the node logs: bug flag on every start, terms with two leaders, conflict truncations.
- Operation counts: ok, info and fail.

## Output

```
results/E6-<date>/E6/<bug>/<fault>/trial-NNN/{result.json,history.jsonl,nemesis.jsonl,n*.log}
results/E6-<date>/E6/<bug>/<fault>/summary.json
results/E6-<date>/E6/correct/<fault>/ , correct-lease/<fault>/ , correct/kill-all-lazyfs/
results/E6-<date>/E6/AUDIT.md      # readiness checks per trial
results/E6-<date>/REPORT.md        # tables from analysis/report.py
```

Earlier result folders are not touched. The script only writes under `$OUT`.

## Acceptance criteria

The run counts as valid only if all of these hold:

1. `make bugtests` reports 7/7 caught on the exact commit used.
2. Every trial in `AUDIT.md` shows the bug flag on every node start, and planned = executed = completed = 3.
3. No fault is shorter than 4.95 s, and no trial has a harness error. Any trial that breaks rule 2 or 3 is reported and rerun as an extra trial, never silently dropped.
4. Controls: 0 Illegal and 0 stale reads in every control cell. One false positive invalidates the detector it came from until it is explained.
5. Each cell has 20 completed trials. Unknown is reported as its own count and never counted as Ok.
6. If a cell has more than 50% Unknown, the Porcupine-only rate is reported as inconclusive for that cell. No CI is built on a denominator under 10.

Expected outcome, from the smoke and the audit (a prediction, not a result):

- High detection: ackbeforefsync × kill-all (stale-read scan) and localread × partition faults.
- Low but non-zero: leasenodrift × clock-partition-mild, and nodedup under pause or partition.
- Near zero: figure8, notruncate, and votenopersist (measured as is). These cells are still worth reporting as harness blind spots, with the deterministic tests showing each bug can fire.
