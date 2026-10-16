#!/usr/bin/env python3
"""Aggregate E1/E2 (2026-10-15). Layout: RAW/<E1|E2>/<cell>/attempt-NNN/trial-001.
A trial without result.json died during checking: it is a checker crash, never
Ok/Unknown/Illegal; its saved history is scanned offline for stale reads and
reported apart. Usage: aggregate.py RAW"""
import glob, json, math, os, re, statistics, sys
INF = float("inf")

def wilson(k, n, z=1.96):
    if n == 0: return (0.0, 1.0)
    p = k/n; d = 1+z*z/n; c = (p+z*z/(2*n))/d
    h = z*math.sqrt(p*(1-p)/n+z*z/(4*n*n))/d
    return max(0, c-h), min(1, c+h)

def stale_scan(hist):
    """Same rule as harness/stale.go; indexed by return time and value so it
    runs in O(n log n) per key."""
    import bisect
    by, n = {}, 0
    for o in hist:
        if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
    ret = lambda o: o["return"] if o["type"] == "ok" else INF
    for ops in by.values():
        writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o.get("cas_ok")))]
        ws = sorted(writes, key=ret); rets = [ret(w) for w in ws]
        pmax, m = [], -1
        for w in ws: m = max(m, w["call"]); pmax.append(m)
        byval = {}
        for w in writes: byval.setdefault(w["value"], []).append(w)
        for r in ops:
            if r["f"] != "read" or r["type"] != "ok": continue
            k = bisect.bisect_left(rets, r["call"])  # writes that returned before the read was invoked
            done = k > 0; C = pmax[k-1] if done else -1
            ok = (not done and r["value"] == -1) or any(w["call"] <= r["return"] and ret(w) >= C for w in byval.get(r["value"], ()))
            if not ok: n += 1
    return n

def jl(p): return [json.loads(l) for l in open(p) if l.strip()] if os.path.exists(p) else []

def durs(ev):
    out, s = [], None
    for e in ev:
        if e["kind"] == "start": s = e["time"]
        elif e["kind"] == "heal" and s is not None: out.append((e["time"]-s)/1e9); s = None
    return out

peaks = {}
al = os.path.join(sys.argv[1], "attempts.log")
if os.path.exists(al):
    for line in open(al):
        m = re.search(r" (E\d)/(\S+) attempt=(\d+) exit=(-?\d+) wall_s=(\d+) peak_rss_kb=(\d+)", line)
        if m: peaks[(m[1], m[2], int(m[3]))] = (int(m[4]), int(m[5]), int(m[6]))

raw = sys.argv[1]
rows, detail = [], []
for exp in ["E1", "E2"]:
    for cd in sorted(glob.glob(f"{raw}/{exp}/*")):
        cell = os.path.basename(cd)
        c = dict(ok=0, red=0, unk=0, ill=0, crash=0, err=0, stale_t=0, stale=0)
        ops = dict(ops=0, ok=0, info=0, fail=0); fpe = []; dd = []; errs = 0; walls = []; rss = []; crash_stale = []; mism = 0
        tds = sorted(glob.glob(f"{cd}/attempt-*/trial-001"))
        for td in tds:
            a = int(td.split("attempt-")[1][:3])
            ev = jl(f"{td}/nemesis.jsonl"); dd += durs(ev); errs += sum(e["kind"] == "error" for e in ev)
            if (exp, cell, a) in peaks: _, w, p = peaks[(exp, cell, a)]; walls.append(w); rss.append(p/1e6)
            rp = f"{td}/result.json"
            if not os.path.exists(rp):
                n = stale_scan(jl(f"{td}/history.jsonl")); c["crash"] += 1; crash_stale.append(n)
                detail.append((exp, cell, a, "checker crash", n, len(jl(f"{td}/history.jsonl")), "-", "-")); continue
            r = json.load(open(rp)); res = r["check"]["result"]; hs = (r.get("stale_reads") or {}).get("reads", 0)
            if hs != stale_scan(jl(f"{td}/history.jsonl")): mism += 1
            if r.get("error"): c["err"] += 1; res = "harness error: " + r["error"][:60]
            else: c[{"Ok": "ok", "Ok (reduced)": "red", "Unknown": "unk", "Illegal": "ill"}[res]] += 1
            if hs: c["stale_t"] += 1; c["stale"] += hs
            k = r["check"]
            for x, y in [("ops", "ops"), ("ok", "ok"), ("info", "info"), ("fail", "fail")]: ops[x] += k.get(y, 0)
            fpe.append((r["faults_planned"], r["faults"], r["faults_completed"]))
            detail.append((exp, cell, a, res, hs, k.get("ops", 0), f"{r['faults_planned']}/{r['faults']}/{r['faults_completed']}", f"{r['wall_s']:.0f}"))
        rows.append((exp, cell, len(tds), c, ops, fpe, dd, errs, walls, rss, crash_stale, mism))

med = lambda xs, f="%.1f": f % statistics.median(xs) if xs else "-"
print("## Outcomes per scenario\n")
print("| Exp | Scenario | Trials | Ok | Ok (reduced) | Unknown | Illegal | Checker crash | Harness error | Correct (Ok + Ok red.) | Inconclusive (Unknown + crash) | Trials with stale reads | Stale reads | Illegal-or-stale [95% CI] |")
print("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|")
for exp, cell, n, c, ops, fpe, dd, errs, walls, rss, cs, mism in rows:
    bad = c["ill"] + c["stale_t"]  # upper bound on trials with a violation
    viol = sum(1 for d in detail if d[0] == exp and d[1] == cell and (d[3] == "Illegal" or d[4] > 0))
    done = n - c["crash"] - c["err"]; lo, hi = wilson(viol, done)
    print(f"| {exp} | {cell} | {n} | {c['ok']} | {c['red']} | {c['unk']} | {c['ill']} | {c['crash']} | {c['err']} | {c['ok']+c['red']}/{n} | {c['unk']+c['crash']}/{n} | {c['stale_t']}/{done} | {c['stale']} | {viol}/{done} [{lo:.2f}, {hi:.2f}] |")
print("\n## Client operations, faults and resources\n")
print("| Exp | Scenario | Ops | ok | fail | info (indeterminate) | info share | Faults planned / executed / completed (per trial) | Fault errors | Shortest / median fault (s) | Median wall (s) | Peak RSS median / max (GB) | Crashed trials: offline stale reads |")
print("|---|---|---|---|---|---|---|---|---|---|---|---|---|")
for exp, cell, n, c, ops, fpe, dd, errs, walls, rss, cs, mism in rows:
    fp = ", ".join(f"{a}/{b}/{d} ×{fpe.count((a,b,d))}" for (a,b,d) in sorted(set(fpe)))
    sh = f"{ops['info']/ops['ops']:.4f}" if ops["ops"] else "-"
    print(f"| {exp} | {cell} | {ops['ops']} | {ops['ok']} | {ops['fail']} | {ops['info']} | {sh} | {fp} | {errs} | {min(dd) if dd else 0:.2f} / {med(dd, '%.2f')} | {med(walls, '%.0f')} | {med(rss, '%.2f')} / {max(rss) if rss else 0:.2f} | {cs or '-'} |")
    if mism: print(f"| | WARNING {exp}/{cell}: {mism} trials where offline scan != harness count | | | | | | | | | | | |")
print("\n## Per trial\n\n| Exp | Scenario | Attempt | Result | Stale reads | Ops | Faults p/e/c | Wall (s) |\n|---|---|---|---|---|---|---|---|")
for d in detail: print("| " + " | ".join(str(x) for x in d) + " |")
