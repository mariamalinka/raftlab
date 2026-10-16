#!/usr/bin/env python3
"""Once a second: CPU% and RSS of every raftkv node and the loadgen process,
plus system CPU pressure (PSI). Writes CSV to argv[1] until killed. With
argv[2] (the loadgen data dir), also logs each change of a node's snapshot
file mtime to argv[1] with .csv replaced by -snapshots.csv."""
import glob, os, sys, time
out = open(sys.argv[1], "w", buffering=1)
out.write("t_unix,pid,name,node,cpu_pct,rss_mb,psi_cpu_some_avg10\n")
hz = os.sysconf("SC_CLK_TCK"); prev = {}
snapdir = sys.argv[2] if len(sys.argv) > 2 else None; snaps = {}
sout = open(sys.argv[1].replace(".csv", "-snapshots.csv"), "w", buffering=1) if snapdir else None
if sout: sout.write("t_seen_unix,file,mtime_unix,size\n")
def psi():
    try:
        return open("/proc/pressure/cpu").readline().split()[1].split("=")[1]
    except Exception: return ""
while True:
    t = time.time(); p = psi()
    for pid in os.listdir("/proc"):
        if not pid.isdigit(): continue
        try:
            cmd = open(f"/proc/{pid}/cmdline", "rb").read().split(b"\0")
            name = os.path.basename(cmd[0].decode())
            if name not in ("raftkv", "loadgen"): continue
            node = cmd[cmd.index(b"-id")+1].decode() if b"-id" in cmd else ""
            st = open(f"/proc/{pid}/stat").read().rsplit(")", 1)[1].split()
            ticks = int(st[11]) + int(st[12]); rss = int(st[21]) * 4096 / 1e6
            cpu = ""
            if pid in prev:
                pt, ptk = prev[pid]; cpu = f"{(ticks-ptk)/hz/(t-pt)*100:.1f}"
            prev[pid] = (t, ticks)
            out.write(f"{t:.3f},{pid},{name},{node},{cpu},{rss:.1f},{p}\n")
        except Exception: pass
    if snapdir:
        for f in glob.glob(os.path.join(snapdir, "**", "snapshot*"), recursive=True):
            try:
                st = os.stat(f)
                if snaps.get(f) != st.st_mtime_ns:
                    snaps[f] = st.st_mtime_ns; sout.write(f"{t:.3f},{os.path.relpath(f, snapdir)},{st.st_mtime_ns/1e9:.3f},{st.st_size}\n")
            except Exception: pass
    time.sleep(max(0, 1 - (time.time() - t)))
