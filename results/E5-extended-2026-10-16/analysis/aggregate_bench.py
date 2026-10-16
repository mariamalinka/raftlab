#!/usr/bin/env python3
"""Part 2: read-path benchmarks. Usage: aggregate_bench.py RAW OUTDIR
Writes OUTDIR/bench-per-rep.csv (every repetition and point, with resources),
OUTDIR/bench.md (medians across the 5 repetitions, min-max) and
OUTDIR/bench.json (data for the figures). A point's measured window is the
20 s before its console line (loadgen prints a point when it finishes)."""
import csv, glob, json, os, re, statistics as st, sys, datetime
raw, out = sys.argv[1], sys.argv[2]

def gc_events(d):
    """GC pauses (wall time, STW ms) from GODEBUG=gctrace=1 output, if present.
    Node lines: offset from the node's 'raft started' time; loadgen lines carry
    the console timestamp."""
    ev = []
    for log in glob.glob(f"{d}/n*.log"):
        t0 = None
        for line in open(log, errors="replace"):
            if '"raft started"' in line and t0 is None:
                t0 = datetime.datetime.fromisoformat(json.loads(line)["time"].replace("Z", "+00:00")[:26] + "+00:00").timestamp()
            m = re.match(r"gc \d+ @([\d.]+)s \d+%: ([\d.]+)\+([\d.]+)\+([\d.]+) ms clock", line)
            if m and t0: ev.append((os.path.basename(log)[:-4], t0 + float(m[1]), float(m[2]) + float(m[4])))
    for line in open(f"{d}/console.log", errors="replace"):
        m = re.match(r"([\d.]+) gc \d+ @([\d.]+)s \d+%: ([\d.]+)\+([\d.]+)\+([\d.]+) ms clock", line)
        if m: ev.append(("loadgen", float(m[1]), float(m[3]) + float(m[5])))
    return ev

def runs(pattern):
    for d in sorted(glob.glob(pattern)):
        pts = [json.loads(l)["result"] for l in open(f"{d}/points.jsonl") if l.strip()]
        ends = {}
        for line in open(f"{d}/console.log"):
            m = re.match(r"([\d.]+) raftkv rate=(\d+) ", line)
            if m: ends[float(m[2])] = float(m[1])
        res = list(csv.DictReader(open(f"{d}/resources.csv"))) if os.path.exists(f"{d}/resources.csv") else []
        snaps = list(csv.DictReader(open(f"{d}/resources-snapshots.csv"))) if os.path.exists(f"{d}/resources-snapshots.csv") else []
        gcs = gc_events(d)
        for p in pts:
            t1 = ends.get(p["offered_ops_s"]); t0 = t1 - 20 if t1 else None
            row = dict(p)
            if t0:
                w = [r for r in res if t0 <= float(r["t_unix"]) <= t1 and r["cpu_pct"]]
                nodes = {}
                for r in w:
                    if r["name"] == "raftkv": nodes.setdefault(r["node"], []).append(float(r["cpu_pct"]))
                lg = [float(r["cpu_pct"]) for r in w if r["name"] == "loadgen"]
                row["cpu_nodes_total_pct"] = round(sum(st.mean(v) for v in nodes.values()), 1) if nodes else ""
                row["cpu_busiest_node_pct"] = round(max(st.mean(v) for v in nodes.values()), 1) if nodes else ""
                row["cpu_loadgen_pct"] = round(st.mean(lg), 1) if lg else ""
                row["rss_max_node_mb"] = max((float(r["rss_mb"]) for r in w if r["name"] == "raftkv"), default="")
                row["psi_cpu_avg10_max"] = max((float(r["psi_cpu_some_avg10"]) for r in w if r["psi_cpu_some_avg10"]), default="")
                row["snapshots_in_window"] = sum(1 for s in snaps if t0 <= float(s["mtime_unix"]) <= t1)
                g = [x for x in gcs if t0 <= x[1] <= t1]
                row["gc_events_in_window"] = len(g) if gcs else ""
                row["gc_max_stw_ms"] = round(max((x[2] for x in g), default=0), 3) if gcs else ""
            row["run_dir"] = os.path.relpath(d, raw)
            yield row

def table(rows, keyf):
    groups = {}
    for r in rows: groups.setdefault(keyf(r), []).append(r)
    return groups

if __name__ == "__main__":
    main = []
    for d in glob.glob(f"{raw}/speed/rf*/*/rep-*"):
        rf = d.split("/speed/rf")[1].split("/")[0]; path = d.split("/")[-2]; rep = int(d.split("rep-")[1])
        for r in runs(d): r.update(mix=rf, path=path, rep=rep); main.append(r)
    diag = []
    for d in glob.glob(f"{raw}/diag-gctrace/*/rep-*"):
        cfg = d.split("/")[-2]; rep = int(d.split("rep-")[1])
        for r in runs(d): r.update(config=cfg, rep=rep); diag.append(r)
    cols = ["mix", "path", "rep", "offered_ops_s", "achieved_ops_s", "ops", "errors", "dropped", "p50_ms", "p99_ms", "p999_ms", "max_ms", "mean_ms",
            "cpu_nodes_total_pct", "cpu_busiest_node_pct", "cpu_loadgen_pct", "rss_max_node_mb", "psi_cpu_avg10_max", "snapshots_in_window", "run_dir"]
    main.sort(key=lambda r: (r["mix"], r["path"], r["offered_ops_s"], r["rep"]))
    with open(f"{out}/bench-per-rep.csv", "w", newline="") as f:
        w = csv.DictWriter(f, cols, extrasaction="ignore"); w.writeheader(); w.writerows(main)
    dcols = ["config", "rep", "offered_ops_s", "p50_ms", "p99_ms", "p999_ms", "max_ms", "errors", "dropped", "snapshots_in_window", "gc_events_in_window", "gc_max_stw_ms", "psi_cpu_avg10_max", "cpu_busiest_node_pct", "cpu_loadgen_pct", "run_dir"]
    diag.sort(key=lambda r: (r["config"], r["rep"]))
    with open(f"{out}/diag-per-rep.csv", "w", newline="") as f:
        w = csv.DictWriter(f, dcols, extrasaction="ignore"); w.writeheader(); w.writerows(diag)
    med = lambda xs: st.median(xs)
    agg = {}
    lines = ["| Mix (read fraction) | Path | Offered (ops/s) | Achieved (median) | p50 ms median [min–max] | p99 ms median [min–max] | p99.9 ms median [min–max] | Errors / dropped (sum) | Node CPU total % (median) | Busiest node CPU % | Reps |", "|---|---|---|---|---|---|---|---|---|---|---|"]
    for (mix, path, rate), rs in sorted(table(main, lambda r: (r["mix"], r["path"], r["offered_ops_s"])).items(), key=lambda kv: (-float(kv[0][0]), ["log", "readindex", "lease"].index(kv[0][1]), kv[0][2])):
        f = lambda k: f"{med([r[k] for r in rs]):.2f} [{min(r[k] for r in rs):.2f}–{max(r[k] for r in rs):.2f}]"
        cpu = [r["cpu_nodes_total_pct"] for r in rs if r.get("cpu_nodes_total_pct") != ""]
        bz = [r["cpu_busiest_node_pct"] for r in rs if r.get("cpu_busiest_node_pct") != ""]
        lines.append(f"| {mix} | {path} | {rate:g} | {med([r['achieved_ops_s'] for r in rs]):.0f} | {f('p50_ms')} | {f('p99_ms')} | {f('p999_ms')} | {sum(r['errors'] for r in rs)} / {sum(r['dropped'] for r in rs)} | {med(cpu):.0f} | {med(bz):.0f} | {len(rs)} |")
        agg.setdefault(mix, {}).setdefault(path, []).append(dict(rate=rate, reps=[{k: r[k] for k in ("rep", "p50_ms", "p99_ms", "p999_ms", "achieved_ops_s", "errors", "dropped")} for r in sorted(rs, key=lambda r: r["rep"])],
            p50=med([r["p50_ms"] for r in rs]), p99=med([r["p99_ms"] for r in rs]), p999=med([r["p999_ms"] for r in rs])))
    # speed-up ratios from medians
    lines += ["", "| Mix | Offered | p50 readindex / lease | p50 log / lease | p99 readindex / lease | p99 log / lease |", "|---|---|---|---|---|---|"]
    for mix in sorted(agg, key=lambda m: -float(m)):
        by = {p: {x["rate"]: x for x in v} for p, v in agg[mix].items()}
        for rate in sorted(by["lease"]):
            L, R, G = by["lease"][rate], by["readindex"][rate], by["log"][rate]
            lines.append(f"| {mix} | {rate:g} | {R['p50']/L['p50']:.1f}× | {G['p50']/L['p50']:.1f}× | {R['p99']/L['p99']:.1f}× | {G['p99']/L['p99']:.1f}× |")
    open(f"{out}/bench.md", "w").write("\n".join(lines) + "\n")
    json.dump(dict(main=agg, diag=[{k: r.get(k) for k in dcols} for r in diag]), open(f"{out}/bench.json", "w"), indent=1)
    print("\n".join(lines))
