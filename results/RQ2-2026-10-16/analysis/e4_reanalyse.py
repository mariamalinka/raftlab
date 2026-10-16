#!/usr/bin/env python3
"""E4 recovery metrics re-derived from the E1 (RaftKV, ReadIndex) and E2 (etcd)
kill-leader and partition-leader histories of 2026-10-15 (commit 80f06fc code,
same as 14bf0f6). Per fault event (3 per trial, 5 s each):
- recovery_ms: fault start -> return of the first write/CAS invoked at or after
  the fault start that the system acknowledged (type ok; CAS only if cas_ok).
- resume_ms: fault start -> first 250 ms bin (after the fault start) from which
  successful writes per bin stay >= 80% of the pre-fault median for 1 s.
- ops invoked during the fault window [start, heal): ok / fail / info (indeterminate).
- election_ms (RaftKV only; etcd logs at warn level do not record elections):
  first 'became leader' with a higher term after the fault start. Node-log
  clock is mapped to history time with an anchor: kill-leader uses the restart
  'raft started' line at heal (min over the trial's heals); partition-leader
  uses the first term-1 'became leader' plus the median kill-leader offset.
Usage: e4_reanalyse.py RAW OUTCSV"""
import json, sys, glob, os, statistics as st, datetime as dt, csv
RAW, OUTCSV = sys.argv[1], sys.argv[2]
def ts(s):
    s = s.rstrip("Z"); a, _, f = s.partition(".")
    return dt.datetime.fromisoformat(a).replace(tzinfo=dt.timezone.utc).timestamp() + float("0." + (f or "0"))
def nodelog(d):
    ev = []
    for f in glob.glob(os.path.join(d, "n*.log")):
        for l in open(f, errors="replace"):
            if not l.startswith('{"time"'): continue
            try: j = json.loads(l)
            except Exception: continue
            if j.get("msg") in ("became leader", "raft started"): ev.append((ts(j["time"]), j["msg"], j.get("node"), j.get("term", 0)))
    return sorted(ev)
rows, offsets = [], []
def analyse(exp, db, fault):
    for td in sorted(glob.glob(os.path.join(RAW, exp, fault, "attempt-*", "trial-*"))):
        nem = [json.loads(l) for l in open(os.path.join(td, "nemesis.jsonl"))]
        rp = os.path.join(td, "result.json")
        res = json.load(open(rp)) if os.path.exists(rp) else {"check": {"result": "no result.json (checker crash)"}}
        H = [json.loads(l) for l in open(os.path.join(td, "history.jsonl"))]
        wok = sorted((h["call"], h["return"]) for h in H if h["type"] == "ok" and (h["f"] == "write" or (h["f"] == "cas" and h.get("cas_ok"))))
        starts = [e["time"] for e in nem if e["kind"] == "start"]; heals = [e["time"] for e in nem if e["kind"] == "heal"]
        anchor = None; ev = nodelog(td) if db == "raftkv" else []
        if db == "raftkv":
            if fault == "kill-leader":
                rs = [t for t, m, n, term in ev if m == "raft started"]; first = min(rs)
                cands = [min(r for r in rs if r >= first + h/1e9 - 0.5) - h/1e9 for h in heals if any(r >= first + h/1e9 - 0.5 for r in rs)]
                anchor = min(cands) if cands else None
                l1 = [t for t, m, n, term in ev if m == "became leader" and term == 1]
                if anchor and l1: offsets.append(anchor - l1[0])
            else:
                l1 = [t for t, m, n, term in ev if m == "became leader" and term == 1]
                anchor = ("L1", l1[0]) if l1 else None
        for i, s in enumerate(starts):
            h = heals[i] if i < len(heals) else s + 5e9
            first = next((r for c, r in wok if c >= s), None)
            # bins of 250 ms for successful-write throughput
            pre = [sum(1 for c, r in wok if s - 2e9 + k*25e7 <= r < s - 2e9 + (k+1)*25e7) for k in range(8)]
            base = st.median(pre); resume = None
            for k in range(0, 80):
                b0 = s + k*25e7
                ok = all(sum(1 for c, r in wok if b0 + j*25e7 <= r < b0 + (j+1)*25e7) >= 0.8*base for j in range(4))
                if ok: resume = k*250; break
            win = [x for x in H if s <= x["call"] < h]
            row = dict(exp=exp, db=db, fault=fault, trial=os.path.relpath(td, RAW), event=i+1, start_s=round(s/1e9, 3),
                       recovery_ms=round((first - s)/1e6, 1) if first else "", resume_ms=resume if resume is not None else "",
                       prefault_writes_per_250ms=base, ok=sum(x["type"] == "ok" for x in win), fail=sum(x["type"] == "fail" for x in win),
                       info=sum(x["type"] == "info" for x in win), check=res["check"]["result"], stale=res.get("stale_reads", {}).get("reads", 0),
                       election_ms="", anchor="")
            if db == "raftkv" and anchor:
                if isinstance(anchor, tuple): a = None; row["anchor"] = "first-leader+median-offset"; row["_L1"] = anchor[1]
                else: a = anchor; row["anchor"] = "restart"
                row["_a"] = a; row["_ev"] = ev; row["_s"] = s
            rows.append(row)
for exp, db in (("E1", "raftkv"), ("E2", "etcd")):
    for fault in ("kill-leader", "partition-leader"): analyse(exp, db, fault)
off = st.median(offsets) if offsets else 0
for r in rows:
    if "_ev" in r:
        a = r["_a"] if r["_a"] is not None else r["_L1"] + off
        s_abs = a + r["_s"]/1e9
        maxterm = max([term for t, m, n, term in r["_ev"] if m == "became leader" and t < s_abs] or [0])
        nl = next((t for t, m, n, term in r["_ev"] if m == "became leader" and t >= s_abs and term > maxterm), None)
        r["election_ms"] = round((nl - s_abs)*1000, 1) if nl else ""
    for k in ("_ev", "_a", "_s", "_L1"): r.pop(k, None)
w = csv.DictWriter(open(OUTCSV, "w"), fieldnames=list(rows[0].keys())); w.writeheader(); w.writerows(rows)
print("offset(kill-leader anchor - first leader) median %.3f s n=%d range %.3f..%.3f" % (off, len(offsets), min(offsets), max(offsets)))
import collections
g = collections.defaultdict(list)
for r in rows: g[(r["db"], r["fault"])].append(r)
for k, v in g.items():
    rec = [r["recovery_ms"] for r in v if r["recovery_ms"] != ""]; res = [r["resume_ms"] for r in v if r["resume_ms"] != ""]
    el = [r["election_ms"] for r in v if r["election_ms"] != ""]
    q = lambda x: "n=%d med %.0f [%.0f-%.0f]" % (len(x), st.median(x), min(x), max(x)) if x else "n/a"
    print(k, "events", len(v), "| recovery", q(rec), "| resume", q(res), "| election", q(el),
          "| fail", sum(r["fail"] for r in v), "info", sum(r["info"] for r in v), "| checks", collections.Counter(r["check"] for r in v[::3]), "stale", sum(r["stale"] for r in v[::3]))
