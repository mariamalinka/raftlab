#!/usr/bin/env python3
# Usage: python3 analysis/stale_reads.py HISTORY.jsonl KEYS(comma) FROM_S TO_S
# Conservative stale-read finder for keys of a register history.
# A read R is flagged when its value cannot be the register's value at any
# point in [R.call, R.return]: the only candidates are writes W (ok writes,
# ok successful CASes, info writes/CASes) with W.call <= R.return that are not
# superseded, i.e. no other write W2 started after W returned and finished
# before R was called. The initial value -1 counts until some write finished.
# Flagged reads are definitely non-linearizable; unflagged ones may still be.
import json, sys
hist = [json.loads(l) for l in open(sys.argv[1])]
keys = [int(k) for k in sys.argv[2].split(",")]
lo, hi = float(sys.argv[3])*1e9, float(sys.argv[4])*1e9
home_of = lambda p: p % 5 + 1
INF = float("inf")
ret = lambda o: o["return"] if o["type"] == "ok" else INF
for key in keys:
    ops = [o for o in hist if o["key"] == key and o["type"] != "fail"]
    writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o["cas_ok"]))]
    flag = []
    for r in ops:
        if r["f"] != "read" or r["type"] != "ok" or not (lo <= r["call"] <= hi): continue
        done = [w["call"] for w in writes if ret(w) < r["call"]]
        C = max(done) if done else -1
        cand = {w["value"] for w in writes if w["call"] <= r["return"] and ret(w) >= C}
        if not done: cand.add(-1)
        if r["value"] not in cand: flag.append(r)
    if flag:
        homes = sorted({home_of(r['process']) for r in flag})
        print(f"key {key}: {len(flag)} stale reads, client homes {homes}, calls {min(r['call'] for r in flag)/1e9:.2f}-{max(r['call'] for r in flag)/1e9:.2f}s")
    else:
        print(f"key {key}: 0")
