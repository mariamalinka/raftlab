#!/usr/bin/env python3
"""Summarise a results tree into Markdown tables (stdlib only).

    python3 analysis/report.py results > results/REPORT.md

Correctness runs (rkharness) are found by summary.json, benchmark runs
(loadgen) by points.jsonl, failover runs by failover.jsonl.
"""
import glob
import json
import os
import statistics
import sys
from collections import defaultdict


def load_jsonl(path):
    with open(path) as f:
        return [json.loads(line) for line in f if line.strip()]


def rel(root, path):
    return os.path.relpath(path, root)


def correctness(root):
    rows = []
    for dirpath, _, files in os.walk(root):
        if "summary.json" in files:
            with open(os.path.join(dirpath, "summary.json")) as f:
                s = json.load(f)
            rows.append((rel(root, dirpath), s))
    rows.sort()
    if not rows:
        return
    by_exp = defaultdict(list)
    for name, s in rows:
        by_exp[name.split(os.sep)[0]].append((name, s))
    for exp, items in sorted(by_exp.items()):
        print(f"\n### {exp}: linearizability\n")
        print("Valid = Ok from the full check; Ok (reduced) = proven Ok by the per-key reduced check after the full check was Unknown;")
        print("Unknown = undecided, never counted as a pass. Faults = planned / executed / completed per trial (min-max), from each")
        print("trial's events; * = planned computed from the config with the current rule (run predates the field; runs before")
        print("commit 9667921 also started a last fault that the end of the run cut short); ! = fewer executed than planned.")
        print("Stale-read scan = trials with provably stale reads (conservative scan, separate from Porcupine; n/a = not recorded).")
        print("Combined detection = trials with Illegal or a provably stale read, over all trials without a harness error (Unknown included).")
        print("Detection [95% CI] is Porcupine alone: Illegal over Valid + Ok (reduced) + Illegal.\n")
        print("| Run | DB | Fault | Bugs | Read path | Trials | Valid | Ok (reduced) | Anomalies | Unknown | Errors | Combined detection [95% CI] | Detection [95% CI] | Median detect (s) | Faults | Stale-read scan | Ops ok / info / fail |")
        print("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|")
        for name, s in items:
            c = s["config"]
            bugs = ",".join(c.get("RaftKV", {}).get("Bugs") or []) or "-"
            rm = c.get("RaftKV", {}).get("ReadMode", "-") if c["DB"] == "raftkv" else ("serializable" if c.get("Etcd", {}).get("Serializable") else "linearizable")
            det = f'{s["detection_rate"]:.2f} [{s["detection_ci95_low"]:.2f}, {s["detection_ci95_high"]:.2f}]'
            md = f'{s["median_time_to_detection_s"]:.1f}' if s.get("median_time_to_detection_s") else "-"
            stale = f'{s["stale_read_trials"]}/{s["trials"]}' if "stale_read_trials" in s else "n/a"
            print(f'| {name} | {c["DB"]} | {c["Fault"]} | {bugs} | {rm} | {s["trials"]} | {s["valid"]} | {s.get("valid_reduced", 0)} | {s["illegal"]} | {s["unknown"]} | {s["errors"]} | {combined_detection(os.path.join(root, name))} | {det} | {md} | {fault_counts(os.path.join(root, name), s)} | {stale} | {s["ok_ops"]} / {s["info_ops"]} / {s["fail_ops"]} |')


def wilson(k, n, z=1.96):
    if n == 0:
        return 0.0, 1.0
    p = k / n
    den = 1 + z * z / n
    centre = (p + z * z / (2 * n)) / den
    half = z * ((p * (1 - p) / n + z * z / (4 * n * n)) ** 0.5) / den
    return max(0.0, centre - half), min(1.0, centre + half)


def combined_detection(run_dir):
    """Trials detected by Porcupine (Illegal) or by the stale-read scan,
    over all trials without a harness error. Unknown trials count in the
    denominator: the scan is conservative, so a trial with no provably stale
    read and no Illegal is not detected."""
    k = n = 0
    for rp in glob.glob(os.path.join(run_dir, "trial-*", "result.json")):
        with open(rp) as f:
            r = json.load(f)
        if r.get("error"):
            continue
        sr = r.get("stale_reads")
        if sr is None:
            return "n/a"
        n += 1
        if r["check"].get("result") == "Illegal" or sr.get("reads", 0) > 0:
            k += 1
    lo, hi = wilson(k, n)
    return f"{k}/{n} = {k / n if n else 0:.2f} [{lo:.2f}, {hi:.2f}]"


def planned_faults(c):
    """Faults that fit in the run: the same rule as harness.PlannedFaults."""
    sch = c.get("Schedule", {})
    warm, dur, gap = sch.get("Warmup", 0), sch.get("Duration", 0), sch.get("Interval", 0)
    if c.get("Fault") == "none" or dur <= 0:
        return 0
    n, t = 0, warm
    while t + dur <= c.get("Duration", 0):
        n += 1
        t += dur + gap
    return n


def fault_counts(run_dir, s):
    """'planned / executed / completed', executed and completed counted from
    the start and heal events in each trial's result.json."""
    executed, completed = [], []
    for t in sorted(os.listdir(run_dir)):
        path = os.path.join(run_dir, t, "result.json")
        if t.startswith("trial-") and os.path.exists(path):
            with open(path) as f:
                ev = json.load(f).get("events") or []
            executed.append(sum(e["kind"] == "start" for e in ev))
            completed.append(sum(e["kind"] == "heal" for e in ev))
    if "faults_planned_per_trial" in s:
        planned, mark = s["faults_planned_per_trial"], ""
    else:
        planned, mark = planned_faults(s["config"]), "*"
    if not executed:
        return f"{planned}{mark} / ? / ?"

    def rng(xs):
        return f"{min(xs)}" if min(xs) == max(xs) else f"{min(xs)}-{max(xs)}"
    flag = " !" if min(executed) < planned else ""
    return f"{planned}{mark} / {rng(executed)} / {rng(completed)}{flag}"


def median_ci(xs):
    """Median with a distribution-free ~95% CI from order statistics (n>=6),
    else min-max."""
    xs = sorted(xs)
    n = len(xs)
    med = statistics.median(xs)
    if n >= 6:
        import math
        k = max(0, int(math.floor((n - 1.96 * math.sqrt(n)) / 2)))
        return med, xs[k], xs[min(n - 1, n - 1 - k)]
    return med, xs[0], xs[-1]


def benchmarks(root):
    runs = []
    for dirpath, _, files in os.walk(root):
        if "points.jsonl" in files:
            runs.append(dirpath)
    if not runs:
        return
    print("\n### Benchmarks: latency vs offered load\n")
    print("Median over repetitions, with min–max (or 95% CI for 6+ reps) in brackets.\n")
    for d in sorted(runs):
        cfg = {}
        if os.path.exists(os.path.join(d, "config.json")):
            with open(os.path.join(d, "config.json")) as f:
                cfg = json.load(f)
        pts = defaultdict(list)
        for row in load_jsonl(os.path.join(d, "points.jsonl")):
            r = row["result"]
            pts[r["offered_ops_s"]].append(r)
        extra = " ".join(x for x in [cfg.get("raftkv_args", ""), "etcd-no-fsync" if cfg.get("etcd_no_fsync") else ""] if x)
        print(f"\n**{rel(root, d)}** ({cfg.get('db', '?')}, reads {cfg.get('read_frac', '?')}, value {cfg.get('value_size', '?')} B, "
              f"{'zipf' if cfg.get('zipf') else 'uniform'}, read path {cfg.get('read_mode', '-')}{', ' + extra if extra else ''})\n")
        print("| Offered (ops/s) | Achieved | p50 ms | p99 ms | p99.9 ms | Errors+dropped |")
        print("|---|---|---|---|---|---|")
        for rate in sorted(pts):
            rs = pts[rate]

            def fmt(key):
                m, lo, hi = median_ci([r[key] for r in rs])
                return f"{m:.2f} [{lo:.2f}–{hi:.2f}]"
            errs = sum(r["errors"] + r["dropped"] for r in rs)
            print(f"| {rate:g} | {statistics.median([r['achieved_ops_s'] for r in rs]):.0f} | {fmt('p50_ms')} | {fmt('p99_ms')} | {fmt('p999_ms')} | {errs} |")


def failover(root):
    runs = []
    for dirpath, _, files in os.walk(root):
        if "failover.jsonl" in files:
            runs.append(dirpath)
    if not runs:
        return
    print("\n### Failover (E4)\n")
    print("| Run | Kills | Unavailability median ms [range] | Catch-up median ms [range] |")
    print("|---|---|---|---|")
    for d in sorted(runs):
        rows = load_jsonl(os.path.join(d, "failover.jsonl"))
        gaps = [r["gap_ms"] for r in rows if r["gap_ms"] >= 0]
        ups = [r["catch_up_ms"] for r in rows if r["catch_up_ms"] >= 0]

        def f(xs):
            if not xs:
                return "-"
            m, lo, hi = median_ci(xs)
            return f"{m:.0f} [{lo:.0f}–{hi:.0f}]"
        print(f"| {rel(root, d)} | {len(rows)} | {f(gaps)} | {f(ups)} |")


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "results"
    print(f"# Results: {root}\n")
    print("Generated by analysis/report.py from raw run folders.")
    correctness(root)
    benchmarks(root)
    failover(root)


if __name__ == "__main__":
    main()
