#!/usr/bin/env python3
"""Aggregate the E5 pilot (2026-10-16). Usage: aggregate.py RAW
Correctness: RAW/correctness/<path>/<condition>/attempt-NNN/trial-001.
A trial without result.json died during checking (checker crash), never Ok,
Unknown or Illegal; its saved history is scanned offline for stale reads.
Speed: RAW/speed/rf<read-frac>/<path>/points.jsonl (loadgen)."""
import bisect, glob, json, math, os, re, statistics, sys
INF = float("inf"); raw = sys.argv[1]

def wilson(k, n, z=1.96):
    if n == 0: return (0.0, 1.0)
    p = k/n; d = 1+z*z/n; c = (p+z*z/(2*n))/d; h = z*math.sqrt(p*(1-p)/n+z*z/(4*n*n))/d
    return max(0, c-h), min(1, c+h)

def stale_scan(hist):
    """harness/stale.go rule; returns (count, first read call time s)."""
    by, n, first = {}, 0, None
    for o in hist:
        if o["type"] != "fail": by.setdefault(o["key"], []).append(o)
    ret = lambda o: o["return"] if o["type"] == "ok" else INF
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
                n += 1; t = r["call"]/1e9; first = t if first is None else min(first, t)
    return n, first

def jl(p): return [json.loads(l) for l in open(p) if l.strip()] if os.path.exists(p) else []
def durs(ev):
    out, s = [], None
    for e in ev:
        if e["kind"] == "start": s = e["time"]
        elif e["kind"] == "heal" and s is not None: out.append((e["time"]-s)/1e9); s = None
    return out
med = lambda xs, f="%.1f": f % statistics.median(xs) if xs else "-"

peaks = {}
for line in open(os.path.join(raw, "attempts.log")):
    m = re.search(r" (\S+/\S+) attempt=(\d+) exit=(-?\d+) wall_s=(\d+) peak_rss_kb=(\d+)", line)
    if m: peaks[(m[1], int(m[2]))] = (int(m[4]), int(m[5]))

order = ["normal", "mild", "severe"]
print("## Correctness per configuration\n")
print("| Read path | Clock condition | Trials | Ok | Ok (reduced) | Unknown | Illegal | Checker crash | Trials with stale reads | Stale reads | Illegal-or-stale [95% CI] | Median Porcupine detection (s) | Median first stale read (s) | Faults p/e/c | Shortest fault (s) | Median wall (s) | Peak RSS max (GB) |")
print("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|")
detail = []
for rm in ["log", "readindex", "lease"]:
    for cond in order:
        cell = f"{rm}/{cond}"; tds = sorted(glob.glob(f"{raw}/correctness/{cell}/attempt-*/trial-001"))
        c = dict(ok=0, red=0, unk=0, ill=0, crash=0, st=0, sr=0, viol=0); det, fst, dd, fpe, walls, rss = [], [], [], [], [], []
        for td in tds:
            a = int(td.split("attempt-")[1][:3]); h = jl(f"{td}/history.jsonl"); n, first = stale_scan(h)
            dd += durs(jl(f"{td}/nemesis.jsonl"))
            if (cell, a) in peaks: walls.append(peaks[(cell, a)][0]); rss.append(peaks[(cell, a)][1]/1e6)
            rp = f"{td}/result.json"
            if not os.path.exists(rp):
                c["crash"] += 1; detail.append((cell, a, "checker crash", f"{n} (offline)", "-", "-", "-")); continue
            r = json.load(open(rp)); k = r["check"]; res = k["result"]; hs = (r.get("stale_reads") or {}).get("reads", 0)
            c[{"Ok": "ok", "Ok (reduced)": "red", "Unknown": "unk", "Illegal": "ill"}[res]] += 1
            if hs: c["st"] += 1; c["sr"] += hs; fst.append(first)
            if res == "Illegal": det.append(k.get("detected_at_s", 0))
            if res == "Illegal" or hs: c["viol"] += 1
            fpe.append(f"{r['faults_planned']}/{r['faults']}/{r['faults_completed']}")
            detail.append((cell, a, res, hs, k.get("detected_at_s", "-") if res == "Illegal" else "-", f"{k.get('ok',0)}/{k.get('fail',0)}/{k.get('info',0)}", ",".join(map(str, k.get("unknown_keys") or [])) or "-"))
        done = len(tds) - c["crash"]; lo, hi = wilson(c["viol"], done)
        print(f"| {rm} | {cond} | {len(tds)} | {c['ok']} | {c['red']} | {c['unk']} | {c['ill']} | {c['crash']} | {c['st']}/{done} | {c['sr']} | {c['viol']}/{done} [{lo:.2f}, {hi:.2f}] | {med(det)} | {med(fst)} | {', '.join(sorted(set(fpe))) or '-'} | {min(dd) if dd else 0:.2f} | {med(walls, '%.0f')} | {max(rss) if rss else 0:.2f} |")
print("\n## Per trial\n\n| Config | Attempt | Result | Stale reads | Porcupine detection (s) | ops ok/fail/info | Unknown keys |\n|---|---|---|---|---|---|---|")
for d in detail: print("| " + " | ".join(str(x) for x in d) + " |")

print("\n## Speed (no faults, open loop; mean of reps, range in brackets)\n")
print("| Read fraction | Read path | Offered (ops/s) | Successful (ops/s) | p50 (ms) | p99 (ms) | p99.9 (ms) | Errors | Dropped | Reps |")
print("|---|---|---|---|---|---|---|---|---|---|")
for rf in sorted(glob.glob(f"{raw}/speed/rf*"), reverse=True):
    for rm in ["log", "readindex", "lease"]:
        rows = jl(f"{rf}/{rm}/points.jsonl"); by = {}
        for row in rows: by.setdefault(row["result"]["offered_ops_s"], []).append(row["result"])
        for rate in sorted(by):
            rs = by[rate]; f = lambda k: f"{statistics.mean(x[k] for x in rs):.2f} [{min(x[k] for x in rs):.2f}–{max(x[k] for x in rs):.2f}]"
            print(f"| {os.path.basename(rf)[2:]} | {rm} | {rate:g} | {f('achieved_ops_s')} | {f('p50_ms')} | {f('p99_ms')} | {f('p999_ms')} | {sum(x['errors'] for x in rs)} | {sum(x['dropped'] for x in rs)} | {len(rs)} |")
