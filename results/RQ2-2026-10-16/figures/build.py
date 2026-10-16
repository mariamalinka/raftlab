#!/usr/bin/env python3
"""Build figure data (data.json) for Figures 1-5 from the RQ2 analysis outputs and inline it into template.html."""
import json, csv, glob, os, sys
ANA, OUT = "/tmp/rk/rq2/ana", "/tmp/rk/rq2/out"
S = json.load(open(f"{ANA}/bench-summary.json"))
def pts(exp, cell):
    r = []
    for s in S:
        if s["exp"] == exp and s["cell"] == cell:
            g = lambda k: s[k]["med"] if s[k] else None
            r.append(dict(rate=s["offered"], n=s["n"], achieved=g("achieved"), p50=g("p50"), p99=g("p99"), p999=g("p999"),
                          cpu=g("cpu_busiest_node_pct"), cputot=g("cpu_nodes_total_pct"), lg=g("cpu_loadgen_pct"), psi=g("psi_cpu_max"), psiio=g("psi_io_max"),
                          rss=g("rss_max_node_mb"), thr=g("threads_max_node"), disk=g("disk_write_mb_s"), sysw=g("write_syscalls_s"),
                          reps=[dict(rep=p["rep"], src=p["src"], p50=p["p50"], p99=p["p99"], p999=p["p999"], achieved=p["achieved"], err=p["errors"]+p["dropped"]) for p in s["per_rep"]]))
    return sorted(r, key=lambda x: x["rate"])
D = dict(e3={m: {db: pts("e3", f"rf{m}/{db}") for db in ("raftkv", "etcd")} for m in ("0.95", "0.5", "0.0")},
         ov={db: pts("overload", db) for db in ("raftkv", "etcd")},
         e7={c: pts("e7", c) for c in ("default", "batch1", "nopipeline", "nofsync-UNSAFE")})
D["e4re"] = [dict(db=r["db"], fault=r["fault"], rec=float(r["recovery_ms"]) if r["recovery_ms"] else None, el=float(r["election_ms"]) if r["election_ms"] else None,
                  res=float(r["resume_ms"]) if r["resume_ms"] else None, trial=r["trial"], ev=r["event"]) for r in csv.DictReader(open(f"{ANA}/e4-reanalysis.csv"))]
D["e4fo"] = []
for db in ("raftkv", "etcd"):
    f = f"{OUT}/e4-killleader/{db}/failover.jsonl"
    if os.path.exists(f):
        for l in open(f):
            j = json.loads(l); D["e4fo"].append(dict(db=db, trial=j["trial"], gap=j["gap_ms"], catchup=j["catch_up_ms"]))
t = open(os.path.join(os.path.dirname(__file__), "template.html")).read()
open(sys.argv[1], "w").write(t.replace("__DATA__", json.dumps(D)))
print("ok", {k: (len(v) if isinstance(v, list) else {kk: len(vv) for kk, vv in v.items()}) for k, v in D.items()})
