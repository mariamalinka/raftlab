#!/usr/bin/env python3
"""Part 1: 60 lease x clock-partition-mild trials. Usage: aggregate_part1.py RAW > part1.md"""
import bisect, glob, json, math, os, re, sys
INF = float("inf"); raw = sys.argv[1]
def wilson(k, n, z=1.96):
    if n == 0: return (0.0, 1.0)
    p = k/n; d = 1+z*z/n; c = (p+z*z/(2*n))/d; h = z*math.sqrt(p*(1-p)/n+z*z/(4*n*n))/d
    return max(0, c-h), min(1, c+h)
def jl(p): return [json.loads(l) for l in open(p) if l.strip()] if os.path.exists(p) else []
def stale(h):
    by, n = {}, 0
    for o in h:
        if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
    ret = lambda o: o["return"] if o["type"] == "ok" else INF
    for ops in by.values():
        ws = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o.get("cas_ok")))]
        s = sorted(ws, key=ret); rets = [ret(w) for w in s]; pm, m = [], -1
        for w in s: m = max(m, w["call"]); pm.append(m)
        bv = {}
        for w in ws: bv.setdefault(w["value"], []).append(w)
        for r in ops:
            if r["f"] != "read" or r["type"] != "ok": continue
            k = bisect.bisect_left(rets, r["call"]); C = pm[k-1] if k else -1
            if not ((not k and r["value"] == -1) or any(w["call"] <= r["return"] and ret(w) >= C for w in bv.get(r["value"], ()))): n += 1
    return n
peaks = {}
for line in open(f"{raw}/attempts.log"):
    m = re.search(r"lease-mild attempt=(\d+) exit=(-?\d+) wall_s=(\d+) peak_rss_kb=(\d+)", line)
    if m: peaks[int(m[1])] = (int(m[2]), int(m[3]), int(m[4]))
rows, cnt = [], dict(Ok=0, red=0, Unknown=0, Illegal=0, crash=0); viol = 0; st_total = 0; mism = 0; durs_all = []
for td in sorted(glob.glob(f"{raw}/lease-mild/attempt-*/trial-001")):
    a = int(td.split("attempt-")[1][:3]); ev = jl(f"{td}/nemesis.jsonl"); d, s = [], None
    for e in ev:
        if e["kind"] == "start": s = e["time"]
        elif e["kind"] == "heal" and s is not None: d.append((e["time"]-s)/1e9); s = None
    durs_all += d; h = jl(f"{td}/history.jsonl"); off = stale(h)
    rc, wall, pk = peaks.get(a, ("?", "?", 0))
    rp = f"{td}/result.json"
    if not os.path.exists(rp):
        cnt["crash"] += 1; rows.append((a, "checker crash", "-", off, "-", d, pk, wall)); continue
    r = json.load(open(rp)); res = r["check"]["result"]; hs = r["stale_reads"]["reads"]
    if hs != off: mism += 1
    cnt["red" if res == "Ok (reduced)" else res] += 1; st_total += hs
    if res == "Illegal" or hs: viol += 1
    rows.append((a, res, ",".join(map(str, r["check"].get("unknown_keys") or [])) or "-", hs, f"{r['faults_planned']}/{r['faults']}/{r['faults_completed']}", d, pk, wall))
n = len(rows); done = n - cnt["crash"]; lo, hi = wilson(viol, done)
pn = cnt["Ok"] + cnt["red"] + cnt["Illegal"]; plo, phi = wilson(cnt["Illegal"], pn)
cfg = json.load(open(glob.glob(f"{raw}/lease-mild/attempt-001/summary.json")[0]))["config"]
print(f"Trials {n}: Ok {cnt['Ok']}, Ok (reduced) {cnt['red']}, Unknown {cnt['Unknown']}, Illegal {cnt['Illegal']}, checker crash {cnt['crash']}")
print(f"Provably stale reads: {st_total} in {viol} trials (offline scan disagreements: {mism})")
print(f"Combined violation rate (Illegal or stale, over {done} completed incl. Unknown): {viol}/{done} = {viol/done:.3f}, Wilson 95% [{lo:.4f}, {hi:.4f}]; rule-of-three ~ {3/done:.3f}")
print(f"Porcupine-only: Illegal {cnt['Illegal']}/{pn} (Unknown excluded), Wilson 95% [{plo:.4f}, {phi:.4f}]")
print(f"Fault durations: {len(durs_all)} faults, min {min(durs_all):.3f} s, max {max(durs_all):.3f} s")
pks = sorted(p for *_, p, _ in rows if p); print(f"Peak checker RSS: median {pks[len(pks)//2]/1e6:.2f} GB, max {pks[-1]/1e6:.2f} GB")
print(f"Config: read_mode={cfg['RaftKV']['ReadMode']} fault={cfg['Fault']} nodes={cfg['Nodes']} duration={cfg['Duration']/1e9}s schedule={cfg['Schedule']} workload={cfg['Workload']} election={cfg['RaftKV']['ElectionTimeout']/1e9}s heartbeat={cfg['RaftKV']['Heartbeat']/1e6}ms check_timeout={cfg['CheckTimeout']/1e9}s")
print("\n| Attempt | Porcupine verdict | Unknown keys | Stale reads | Faults p/e/c | Fault durations (s) | Peak checker RSS (GB) | Wall (s) |\n|---|---|---|---|---|---|---|---|")
for a, res, uk, hs, fpe, d, pk, wall in rows:
    print(f"| {a} | {res} | {uk} | {hs} | {fpe} | {', '.join(f'{x:.2f}' for x in d)} | {pk/1e6:.2f} | {wall} |")
