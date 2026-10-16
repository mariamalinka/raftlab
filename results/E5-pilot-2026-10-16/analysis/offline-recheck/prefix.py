#!/usr/bin/env python3
"""Print 'key cut_ns' for the first provably stale read of each key (cut at its return)."""
import bisect, json, sys
INF = float("inf")
h = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
by = {}
for o in h:
    if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
ret = lambda o: o["return"] if o["type"] == "ok" else INF
for key, ops in sorted(by.items()):
    writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o.get("cas_ok")))]
    ws = sorted(writes, key=ret); rets = [ret(w) for w in ws]; pmax, m = [], -1
    for w in ws: m = max(m, w["call"]); pmax.append(m)
    byval = {}
    for w in writes: byval.setdefault(w["value"], []).append(w)
    first = None
    for r in ops:
        if r["f"] != "read" or r["type"] != "ok": continue
        k = bisect.bisect_left(rets, r["call"]); done = k > 0; C = pmax[k-1] if done else -1
        if not ((not done and r["value"] == -1) or any(w["call"] <= r["return"] and ret(w) >= C for w in byval.get(r["value"], ()))):
            if first is None or r["return"] < first: first = r["return"]
    if first is not None: print(key, first)
