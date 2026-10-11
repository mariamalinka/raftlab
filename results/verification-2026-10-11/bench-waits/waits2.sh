#!/bin/bash
# One loadgen invocation per (mix, rate) so the cumulative counters belong to one load point.
# Node logs live in /tmp/raftlab-bench and are deleted at teardown, so copy them every second.
cd /mnt/project-files/research/raftkv
OUT=results/verification-2026-10-11/bench-waits/per-rate
for mix in 0.5 0; do for rate in 1000 4000 8000; do
  d=$OUT/r$mix-$rate; mkdir -p $d
  /tmp/bench-cfa86fa/loadgen -db raftkv -bin /tmp/bench-waits -read-frac $mix -rates $rate -reps 1 -out $d > $d/stdout.txt 2>&1 &
  p=$!
  while kill -0 $p 2>/dev/null; do for n in 1 2 3 4 5; do grep INSTRWAIT /tmp/raftlab-bench/n$n.log 2>/dev/null | tail -1 > $d/.n$n.tmp && [ -s $d/.n$n.tmp ] && mv $d/.n$n.tmp $d/instr-n$n.txt; done; sleep 1; done
  rm -f $d/.n*.tmp
done; done
