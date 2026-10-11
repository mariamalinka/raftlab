#!/usr/bin/env python3
# Per kill-all fault in a trial directory:
#  acked  = ok writes + ok CASes that returned before the kill (each is >= 1 log entry)
#  maxlast= largest last_index any node reloaded on the restart after that kill
#  deficit= acked - maxlast; > 0 proves acknowledged entries were lost (lower bound:
#           no-ops, retries and failed-then-retried entries only make the log longer)
#  keys   = keys with acked writes before the kill; reads = ok reads of them after heal
#  stale  = reads flagged by the conservative stale-read rule after the heal
import json, sys, os, collections
d = sys.argv[1]
hist = [json.loads(l) for l in open(os.path.join(d, "history.jsonl"))]
nem = [json.loads(l) for l in open(os.path.join(d, "nemesis.jsonl"))]
starts = [e["time"] for e in nem if e["kind"] == "start"]
heals = [e["time"] for e in nem if e["kind"] == "heal"]
restarts = collections.defaultdict(list)
for n in range(1, 6):
    for l in open(os.path.join(d, f"n{n}.log")):
        if '"raft started"' in l:
            restarts[n].append(json.loads(l)["last_index"])
INF = float("inf")
ret = lambda o: o["return"] if o["type"] == "ok" else INF
def stale(key, lo, hi):
    ops = [o for o in hist if o["key"] == key and o["type"] != "fail"]
    writes = [o for o in ops if o["f"] == "write" or (o["f"] == "cas" and (o["type"] == "info" or o["cas_ok"]))]
    n = 0
    for r in ops:
        if r["f"] != "read" or r["type"] != "ok" or not (lo <= r["call"] <= hi): continue
        done = [w["call"] for w in writes if ret(w) < r["call"]]
        C = max(done) if done else -1
        cand = {w["value"] for w in writes if w["call"] <= r["return"] and ret(w) >= C}
        if not done: cand.add(-1)
        if r["value"] not in cand: n += 1
    return n
out = []
for k, (s, h) in enumerate(zip(starts, heals)):
    acked = sum(1 for o in hist if o["type"] == "ok" and o["f"] in ("write", "cas") and o["return"] < s)
    lasts = [restarts[n][k + 1] for n in restarts if len(restarts[n]) > k + 1]
    maxlast = max(lasts) if lasts else None
    keys = sorted({o["key"] for o in hist if o["type"] == "ok" and o["f"] in ("write", "cas") and s - 1e9 < o["return"] < s})
    nxt = starts[k + 1] if k + 1 < len(starts) else INF
    reads = sum(1 for o in hist if o["key"] in keys and o["f"] == "read" and o["type"] == "ok" and h <= o["call"] < nxt)
    st = sum(stale(key, h, nxt) for key in keys)
    out.append(dict(kill=k + 1, acked=acked, maxlast=maxlast, deficit=None if maxlast is None else acked - maxlast,
                    keys=keys, reads_after=reads, stale_after=st))
res = json.load(open(os.path.join(d, "result.json")))["check"]["result"]
print(json.dumps(dict(trial=os.path.basename(d), result=res, kills=out)))
