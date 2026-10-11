# Per-key search-difficulty proxies for a register history.
# For each key: ops (non-fail), info ops, mean/max concurrency of ok ops,
# and "open window": count of ok ops overlapping each info op's lifetime
# (info ops never return, so Porcupine treats them as pending to the end).
import json,sys,collections,bisect
h=[json.loads(l) for l in open(sys.argv[1])]
by=collections.defaultdict(list)
for o in h:
    if o['type']!='fail': by[o['key']].append(o)
end=max(o.get('return',0) or 0 for o in h)
for k,ops in sorted(by.items()):
    ev=[]
    for o in ops:
        r=o['return'] if o['type']=='ok' else end
        ev+=[(o['call'],1),(r,-1)]
    ev.sort()
    cur=mx=0; area=0; last=ev[0][0]
    for t,d in ev:
        area+=cur*(t-last); last=t; cur+=d; mx=max(mx,cur)
    span=ev[-1][0]-ev[0][0]
    info=[o for o in ops if o['type']=='info']
    # max latency of ok ops (ms)
    lat=sorted((o['return']-o['call'])/1e6 for o in ops if o['type']=='ok')
    print(f"key {k}: ops={len(ops)} info={len(info)} meanconc={area/span:.1f} maxconc={mx} lat_p99={lat[int(.99*len(lat))]:.0f}ms lat_max={lat[-1]:.0f}ms")
