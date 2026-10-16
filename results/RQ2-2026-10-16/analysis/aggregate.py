#!/usr/bin/env python3
"""Aggregate RQ2 bench runs. Usage: aggregate.py OUT ANADIR
Walks OUT/{e3,overload,e7}/<...>/rep-N (and the E5-extension readindex 95/5
reps passed via E5RAW env) and writes per-rep CSV + medians JSON. Resource
metrics are taken over each point's measured window (the 20 s before its
console line): node CPU sum (median of 1 s samples), busiest node CPU,
loadgen CPU, max node RSS, max node threads, max PSI cpu/io, node disk write
MB/s and write syscalls/s (sum over nodes, from /proc/pid/io deltas)."""
import json, os, sys, glob, csv, statistics as st, collections
OUT, ANA = sys.argv[1], sys.argv[2]; os.makedirs(ANA, exist_ok=True)
def runs():
    for d in sorted(glob.glob(os.path.join(OUT, "**", "rep-*"), recursive=True)):
        if os.path.exists(os.path.join(d, "points.jsonl")): yield d, os.path.relpath(d, OUT)
    e5 = os.environ.get("E5RAW")
    if e5:
        for d in sorted(glob.glob(os.path.join(e5, "speed/rf0.95/readindex/rep-*"))):
            yield d, "e3/rf0.95/raftkv-e5ext/" + os.path.basename(d)
def res(d, t0, t1):
    f = os.path.join(d, "resources.csv")
    if not os.path.exists(f): return {}
    S = collections.defaultdict(list)
    for r in csv.DictReader(open(f)):
        t = float(r["t_unix"])
        if t0 <= t <= t1: S[round(t)].append(r)
    nodes = ("raftkv", "etcd")
    tot, busy, lg, rss, thr, pc, pi = [], [], [], [], [], [], []
    wb = collections.defaultdict(list); sc = collections.defaultdict(list)
    for t, rs in sorted(S.items()):
        nc = [float(r["cpu_pct"]) for r in rs if r["name"] in nodes and r["cpu_pct"]]
        if nc: tot.append(sum(nc)); busy.append(max(nc))
        lg += [float(r["cpu_pct"]) for r in rs if r["name"] == "loadgen" and r["cpu_pct"]]
        rss += [float(r["rss_mb"]) for r in rs if r["name"] in nodes]
        thr += [int(r["threads"]) for r in rs if r["name"] in nodes and r.get("threads")]
        for r in rs:
            if r.get("psi_cpu_some_avg10"): pc.append(float(r["psi_cpu_some_avg10"]))
            if r.get("psi_io_some_avg10"): pi.append(float(r["psi_io_some_avg10"]))
            if r["name"] in nodes and r.get("write_bytes"):
                wb[r["pid"]].append((float(r["t_unix"]), int(r["write_bytes"]))); sc[r["pid"]].append((float(r["t_unix"]), int(r["syscw"])))
    rate = lambda D: sum((v[-1][1]-v[0][1])/(v[-1][0]-v[0][0]) for v in D.values() if len(v) > 1 and v[-1][0] > v[0][0])
    m = lambda x: round(st.median(x), 1) if x else ""
    return dict(cpu_nodes_total_pct=m(tot), cpu_busiest_node_pct=m(busy), cpu_loadgen_pct=m(lg), rss_max_node_mb=round(max(rss), 1) if rss else "",
                threads_max_node=max(thr) if thr else "", psi_cpu_max=max(pc) if pc else "", psi_io_max=max(pi) if pi else "",
                disk_write_mb_s=round(rate(wb)/1e6, 2) if wb else "", write_syscalls_s=round(rate(sc)) if sc else "")
rows = []
for d, rel in runs():
    parts = rel.split("/"); exp = parts[0]
    cons = [l.split(" ", 1) for l in open(os.path.join(d, "console.log")) if "rate=" in l]
    ctime = {}
    for t, l in cons:
        rate = float(l.split("rate=")[1].split()[0]); ctime[rate] = float(t)
    for l in open(os.path.join(d, "points.jsonl")):
        r = json.loads(l)["result"]; t1 = ctime.get(float(r["offered_ops_s"]))
        row = dict(exp=exp, cell="/".join(parts[1:-1]), rep=parts[-1], offered=r["offered_ops_s"], achieved=r["achieved_ops_s"],
                   ops=r["ops"], errors=r["errors"], dropped=r["dropped"], p50=r["p50_ms"], p99=r["p99_ms"], p999=r["p999_ms"], max=r["max_ms"])
        if t1: row.update(res(d, t1 - 20, t1))
        row["run_dir"] = rel; rows.append(row)
keys = []
for r in rows:
    for k in r:
        if k not in keys: keys.append(k)
w = csv.DictWriter(open(os.path.join(ANA, "bench-per-rep.csv"), "w"), fieldnames=keys); w.writeheader(); w.writerows(rows)
G = collections.defaultdict(list)
for r in rows: G[(r["exp"], r["cell"].replace("raftkv-e5ext", "raftkv"), r["offered"])].append(r)
summ = []
for (exp, cell, off), v in sorted(G.items()):
    def agg(k):
        x = [float(r[k]) for r in v if r.get(k, "") != ""]
        return dict(med=round(st.median(x), 3), min=min(x), max=max(x)) if x else None
    summ.append(dict(exp=exp, cell=cell, offered=off, n=len(v), reps=[r["rep"] + ("(e5ext)" if "e5ext" in r["cell"] else "") for r in v],
                     **{k: agg(k) for k in ("achieved", "p50", "p99", "p999", "errors", "dropped", "cpu_nodes_total_pct", "cpu_busiest_node_pct", "cpu_loadgen_pct", "rss_max_node_mb", "threads_max_node", "psi_cpu_max", "psi_io_max", "disk_write_mb_s", "write_syscalls_s")},
                     per_rep=[dict(rep=r["rep"], src="e5ext" if "e5ext" in r["cell"] else "new", p50=r["p50"], p99=r["p99"], p999=r["p999"], achieved=r["achieved"], errors=r["errors"], dropped=r["dropped"]) for r in v]))
json.dump(summ, open(os.path.join(ANA, "bench-summary.json"), "w"), indent=1)
f = lambda a: "" if a is None else ("%.4g" % a["med"] if a["min"] == a["max"] else "%.4g [%.4g–%.4g]" % (a["med"], a["min"], a["max"]))
with open(os.path.join(ANA, "bench.md"), "w") as o:
    o.write("| Exp | Cell | Offered | n | Achieved | p50 ms | p99 ms | p99.9 ms | Errors | Dropped | Node CPU % | Busiest % | Loadgen % | Max RSS MB | PSI cpu | PSI io | Disk MB/s | Write syscalls/s |\n|" + "---|"*18 + "\n")
    for s in summ:
        o.write("| %s | %s | %s | %d | %s |\n" % (s["exp"], s["cell"], s["offered"], s["n"], " | ".join(f(s[k]) for k in ("achieved", "p50", "p99", "p999", "errors", "dropped", "cpu_nodes_total_pct", "cpu_busiest_node_pct", "cpu_loadgen_pct", "rss_max_node_mb", "psi_cpu_max", "psi_io_max", "disk_write_mb_s", "write_syscalls_s"))))
print(open(os.path.join(ANA, "bench.md")).read())
