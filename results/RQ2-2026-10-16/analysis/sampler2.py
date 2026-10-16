#!/usr/bin/env python3
"""Once a second: CPU%, RSS, OS threads, cumulative disk write bytes and
fsync-relevant syscall counts (syscw) of every raftkv / etcd node and loadgen,
plus system CPU and IO pressure (PSI some avg10). Writes CSV to argv[1] until killed."""
import os, sys, time
out = open(sys.argv[1], "w", buffering=1)
out.write("t_unix,pid,name,node,cpu_pct,rss_mb,threads,write_bytes,syscw,psi_cpu_some_avg10,psi_io_some_avg10\n")
hz = os.sysconf("SC_CLK_TCK"); prev = {}
def psi(k):
    try: return open(f"/proc/pressure/{k}").readline().split()[1].split("=")[1]
    except Exception: return ""
while True:
    t = time.time(); pc = psi("cpu"); pi = psi("io")
    for pid in os.listdir("/proc"):
        if not pid.isdigit(): continue
        try:
            cmd = open(f"/proc/{pid}/cmdline", "rb").read().split(b"\0")
            name = os.path.basename(cmd[0].decode())
            if name not in ("raftkv", "loadgen", "etcd"): continue
            node = ""
            if b"-id" in cmd: node = cmd[cmd.index(b"-id")+1].decode()
            elif b"--name" in cmd: node = cmd[cmd.index(b"--name")+1].decode()
            st = open(f"/proc/{pid}/stat").read().rsplit(")", 1)[1].split()
            ticks = int(st[11]) + int(st[12]); rss = int(st[21]) * 4096 / 1e6; thr = st[17]
            io = dict(l.split(": ") for l in open(f"/proc/{pid}/io").read().split("\n") if ": " in l)
            cpu = ""
            if pid in prev:
                pt, ptk = prev[pid]; cpu = f"{(ticks-ptk)/hz/(t-pt)*100:.1f}"
            prev[pid] = (t, ticks)
            out.write(f"{t:.3f},{pid},{name},{node},{cpu},{rss:.1f},{thr},{io.get('write_bytes','')},{io.get('syscw','')},{pc},{pi}\n")
        except Exception: pass
    time.sleep(max(0, 1 - (time.time() - t)))
