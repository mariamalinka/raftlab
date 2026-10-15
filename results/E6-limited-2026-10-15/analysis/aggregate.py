#!/usr/bin/env python3
"""Aggregate the limited E6 run (2026-10-15) over its two layouts:
raw/E6/<cell>/trial-NNN (first process, -trials 20) and
raw/E6/<cell>-single/attempt-NNN/trial-001 (resume, -trials 1).
A trial without result.json was killed (OOM) during checking: it is counted
as a checker crash, never as Ok, Unknown or Illegal. Its history is scanned
offline for stale reads with the same rule as harness/stale.go, reported
apart. Usage: aggregate.py RAW_DIR"""
import glob, json, math, os, statistics, sys

INF = float("inf")

def wilson(k, n, z=1.96):
    if n == 0: return (0.0, 1.0)
    p = k / n; d = 1 + z*z/n; c = (p + z*z/(2*n)) / d
    h = z * math.sqrt(p*(1-p)/n + z*z/(4*n*n)) / d
    return max(0, c-h), min(1, c+h)

def stale_scan(hist):
    """Port of analysis/stale_reads.py over every key: count and earliest call (s)."""
    n, first = 0, None
    by = {}
    for o in hist:
        if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
    ret = lambda o: o["return"] if o["type"] == "ok" else INF
    for ops in by.values():
        writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o.get("cas_ok")))]
        for r in ops:
            if r["f"] != "read" or r["type"] != "ok": continue
            done = [w["call"] for w in writes if ret(w) < r["call"]]
            C = max(done) if done else -1
            cand = {w["value"] for w in writes if w["call"] <= r["return"] and ret(w) >= C}
            if not done: cand.add(-1)
            if r["value"] not in cand:
                n += 1; t = r["call"]/1e9; first = t if first is None else min(first, t)
    return n, first

def node_bugs(td):
    starts = ok = 0
    for l in glob.glob(os.path.join(td, "n*.log")):
        for line in open(l, errors="replace"):
            if '"raft started"' in line:
                starts += 1; e = json.loads(line); ok += bool(e.get("bugs"))
    return starts, ok

def fault_durs(ev):
    out, s = [], None
    for e in ev:
        if e["kind"] == "start": s = e["time"]
        elif e["kind"] == "heal" and s is not None: out.append((e["time"]-s)/1e9); s = None
    return out

raw = sys.argv[1]
cells = {}
for cell in ["ackbeforefsync/kill-all", "correct-lazyfs/kill-all", "localread/partition-leader", "correct/partition-leader"]:
    tds = sorted(glob.glob(f"{raw}/E6/{cell}/trial-*")) + sorted(glob.glob(f"{raw}/E6/{cell}-single/attempt-*/trial-001"))
    cells[cell] = tds

print("| Cell | Attempts | Completed | Ok | Ok (reduced) | Unknown | Illegal | Checker crash (OOM) | Trials with stale reads | Stale reads | Combined detection [95% CI] | Porcupine-only detection [95% CI] | Median detection, Porcupine (s) | Median first stale read (s) | Faults planned / executed / completed | Shortest fault (s) | Bug flag on every node start |")
print("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|")
detail = []
for cell, tds in cells.items():
    c = dict(ok=0, red=0, unk=0, ill=0, crash=0, stale_t=0, stale=0, det=0)
    pt, st_t, fp = [], [], []
    durs = []; flags_ok = True; mism = 0; starts_total = 0; crash_stale = []
    for td in tds:
        h = [json.loads(l) for l in open(os.path.join(td, "history.jsonl")) if l.strip()]
        n, first = stale_scan(h)
        ev = [json.loads(l) for l in open(os.path.join(td, "nemesis.jsonl")) if l.strip()]
        durs += fault_durs(ev)
        rp = os.path.join(td, "result.json")
        if not os.path.exists(rp):
            c["crash"] += 1; crash_stale.append(n)
            detail.append((cell, td, "checker crash (OOM)", n, "-"))
            continue
        r = json.load(open(rp)); res = r["check"]["result"]
        hs = r["stale_reads"]["reads"]
        if hs != n: mism += 1
        key = {"Ok": "ok", "Ok (reduced)": "red", "Unknown": "unk", "Illegal": "ill"}[res]
        c[key] += 1
        if hs > 0: c["stale_t"] += 1; c["stale"] += hs; st_t.append(first)
        if res == "Illegal": pt.append(r["check"]["detected_at_s"])
        if res == "Illegal" or hs > 0: c["det"] += 1
        fp.append((r["faults_planned"], r["faults"], r["faults_completed"]))
        s, b = node_bugs(td); starts_total += s
        if "correct" not in cell and b != s: flags_ok = False
        detail.append((cell, td, res, hs, f'{r["check"].get("detected_at_s") or "-"}'))
    done = c["ok"] + c["red"] + c["unk"] + c["ill"]
    k, n = c["det"], done; lo, hi = wilson(k, n)
    pn = c["ok"] + c["red"] + c["ill"]; plo, phi = wilson(c["ill"], pn)
    med = lambda xs: f"{statistics.median(xs):.1f}" if xs else "-"
    fps = sorted(set(fp))
    flag = "n/a (no bug)" if "correct" in cell else ("yes" if flags_ok else "NO") + f" ({starts_total} starts in {done} trials)"
    print(f"| {cell} | {len(tds)} | {done} | {c['ok']} | {c['red']} | {c['unk']} | {c['ill']} | {c['crash']} | {c['stale_t']}/{done} | {c['stale']} | "
          f"{k}/{n} = {k/n if n else 0:.2f} [{lo:.2f}, {hi:.2f}] | {c['ill']}/{pn} = {c['ill']/pn if pn else 0:.2f} [{plo:.2f}, {phi:.2f}] | {med(pt)} | {med(st_t)} | "
          f"{', '.join('%d/%d/%d' % x for x in fps)} (all completed trials) | {min(durs):.2f} of 5 | {flag} |")
    if crash_stale: print(f"|  | crashed trials, offline stale-read scan of saved history: {crash_stale} | | | | | | | | | | | | | | | |")
    if mism: print(f"|  | WARNING: {mism} trials where the offline scan disagrees with the harness count | | | | | | | | | | | | | | | |")
print("\n## Per trial\n\n| Cell | Trial dir | Result | Stale reads | Porcupine detection (s) |\n|---|---|---|---|---|")
for cell, td, res, n, d in detail:
    print(f"| {cell} | {os.path.relpath(td, raw)} | {res} | {n} | {d} |")
