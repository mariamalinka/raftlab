#!/usr/bin/env python3
"""Per-trial readiness audit for E6 runs (stdlib only).

    python3 analysis/e6_audit.py results/E6 > results/E6/AUDIT.md

For every trial under the given tree it reports what the harness result
alone does not show:

- whether every node start logged the bugs the cell asked for (the
  injection flag reached the process);
- how long each fault really lasted, from nemesis.jsonl, against the
  configured fault duration (a fault cut short is flagged);
- evidence from the node logs that a bug's precondition occurred: terms
  with two leaders (votenopersist) and conflicting-suffix truncations
  (notruncate's code path is only reached on a conflict);
- the checker outcome and the stale-read scan, kept apart.

It reads only result files and node logs, and changes nothing.
"""
import glob
import json
import os
import sys
from collections import defaultdict

SLACK_S = 0.05  # a heal logged this much early still counts as a full fault


def load_jsonl(path):
    if not os.path.exists(path):
        return []
    with open(path, errors="replace") as f:
        out = []
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                out.append(json.loads(line))
            except json.JSONDecodeError:
                pass
        return out


def node_evidence(trial_dir, want_bugs):
    leaders = defaultdict(set)
    truncations = 0
    starts = 0
    starts_without_bugs = 0
    for log in sorted(glob.glob(os.path.join(trial_dir, "n*.log"))):
        node = os.path.basename(log)[1:-4]
        for e in load_jsonl(log):
            msg = e.get("msg")
            if msg == "raft started":
                starts += 1
                if not set(want_bugs) <= set(e.get("bugs") or []):
                    starts_without_bugs += 1
            elif msg == "became leader":
                leaders[e.get("term")].add(node)
            elif msg == "truncated conflicting suffix":
                truncations += 1
    two = {t: sorted(n) for t, n in leaders.items() if len(n) > 1}
    return starts, starts_without_bugs, two, truncations


def fault_durations(events):
    out, start = [], None
    for e in events:
        if e["kind"] == "start":
            start = e["time"]
        elif e["kind"] == "heal" and start is not None:
            out.append((e["time"] - start) / 1e9)
            start = None
    return out


def main(root):
    cells = sorted({os.path.dirname(p) for p in glob.glob(os.path.join(root, "**", "summary.json"), recursive=True)})
    print("| Cell | Trial | Bugs on every start | Faults planned / executed / completed | Shortest fault (s) of configured | Two leaders in a term | Suffix truncations | Checker | Stale reads |")
    print("|---|---|---|---|---|---|---|---|---|")
    problems = 0
    for cell in cells:
        with open(os.path.join(cell, "summary.json")) as f:
            cfg = json.load(f)["config"]
        want = cfg["RaftKV"].get("Bugs") or []
        conf = cfg["Schedule"]["Duration"] / 1e9
        for td in sorted(glob.glob(os.path.join(cell, "trial-*"))):
            rp = os.path.join(td, "result.json")
            if not os.path.exists(rp):
                continue
            with open(rp) as f:
                r = json.load(f)
            starts, missing, two, trunc = node_evidence(td, want)
            durs = fault_durations(r.get("events") or [])
            short = [d for d in durs if d < conf - SLACK_S]
            planned = r.get("faults_planned", "?")
            flags = []
            if missing:
                flags.append(f"{missing}/{starts} starts without bugs")
            if isinstance(planned, int) and r.get("faults", 0) < planned:
                flags.append("executed < planned")
            if short:
                flags.append(f"{len(short)} short")
            if r.get("error"):
                flags.append("error: " + r["error"][:60])
            problems += bool(flags)
            stale = (r.get("stale_reads") or {}).get("reads", "-")
            shortest = f"{min(durs):.2f}" if durs else "-"
            note = " **" + "; ".join(flags) + "**" if flags else ""
            print(f"| {os.path.relpath(cell, root)} | {r['trial']} | "
                  f"{'yes' if not missing else 'NO'} ({starts} starts) | "
                  f"{planned} / {r.get('faults', '?')} / {r.get('faults_completed', '?')} | "
                  f"{shortest} of {conf:g} | {two or '-'} | {trunc} | "
                  f"{r['check'].get('result')} | {stale} |{note}")
    print(f"\nTrials with a readiness problem: {problems}")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "results")
