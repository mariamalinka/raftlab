#!/usr/bin/env python3
"""Offline stale-read scan of a saved history (same rule as harness/stale.go,
ported in analysis/stale_reads.py). Exit 1 if any provably stale read."""
import json, sys
INF = float("inf")
hist = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
by = {}
for o in hist:
    if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
ret = lambda o: o["return"] if o["type"] == "ok" else INF
n = 0
for ops in by.values():
    writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o.get("cas_ok")))]
    for r in ops:
        if r["f"] != "read" or r["type"] != "ok": continue
        done = [w["call"] for w in writes if ret(w) < r["call"]]
        C = max(done) if done else -1
        cand = {w["value"] for w in writes if w["call"] <= r["return"] and ret(w) >= C}
        if not done: cand.add(-1)
        if r["value"] not in cand: n += 1
print(f"offline stale reads: {n} in {len(hist)} ops")
sys.exit(1 if n else 0)
