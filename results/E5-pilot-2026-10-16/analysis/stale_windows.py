#!/usr/bin/env python3
"""For each provably stale read in an E5 trial: when it happened relative to the
fault that was active, which node served it (client home node = process % 5 + 1),
and whether that node was the slowed, isolated leader. Usage: stale_windows.py TRIAL_DIR..."""
import bisect, json, re, sys
INF = float("inf")
def jl(p): return [json.loads(l) for l in open(p) if l.strip()]
for td in sys.argv[1:]:
    h = jl(f"{td}/history.jsonl"); ev = jl(f"{td}/nemesis.jsonl")
    faults = []; s = None
    for e in ev:
        if e["kind"] == "start": s = e
        elif e["kind"] == "heal" and s:
            m = re.search(r"on n(\d+)", s["desc"]); faults.append((s["time"]/1e9, e["time"]/1e9, int(m[1]) if m else None, s["desc"])); s = None
    by = {}
    for o in h:
        if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
    ret = lambda o: o["return"] if o["type"] == "ok" else INF
    stale = []
    for ops in by.values():
        writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o.get("cas_ok")))]
        ws = sorted(writes, key=ret); rets = [ret(w) for w in ws]; pmax, m = [], -1
        for w in ws: m = max(m, w["call"]); pmax.append(m)
        byval = {}
        for w in writes: byval.setdefault(w["value"], []).append(w)
        for r in ops:
            if r["f"] != "read" or r["type"] != "ok": continue
            k = bisect.bisect_left(rets, r["call"]); done = k > 0; C = pmax[k-1] if done else -1
            if not ((not done and r["value"] == -1) or any(w["call"] <= r["return"] and ret(w) >= C for w in byval.get(r["value"], ()))):
                stale.append(r)
    print(f"{td}: {len(stale)} stale reads")
    for f0, f1, slow, desc in faults:
        rs = [r for r in stale if f0 <= r["call"]/1e9 <= f1 + 1]
        nodes = sorted({r["process"] % 5 + 1 for r in rs})
        if rs:
            t = [r["call"]/1e9 - f0 for r in rs]
            print(f"  fault {f0:.1f}-{f1:.1f}s [{desc}]: {len(rs)} stale, served by node(s) {nodes}, slowed leader n{slow}, {min(t):.2f}-{max(t):.2f}s after fault start")
        else:
            print(f"  fault {f0:.1f}-{f1:.1f}s [{desc}]: 0 stale")
    outside = [r for r in stale if not any(f0 <= r["call"]/1e9 <= f1 + 1 for f0, f1, _, _ in faults)]
    print(f"  stale reads outside any fault window (+1 s): {len(outside)}")
    # new leader elections during each fault, from node logs
    for f0, f1, slow, _ in faults:
        el = []
        for n in range(1, 6):
            try:
                for e in jl(f"{td}/n{n}.log"):
                    if e.get("msg") == "became leader":
                        el.append(n)
            except FileNotFoundError: pass
    print()
