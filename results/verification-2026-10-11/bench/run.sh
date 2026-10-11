#!/bin/bash
# Latency/throughput sweep: unfixed (9667921) vs fixed (cfa86fa) raftkv vs etcd 3.7.2.
# Same machine, 5 nodes, election 1s, heartbeat 100ms, default batching/pipelining,
# fsync on. Repetitions are separate invocations, interleaved across systems.
set -u
cd /mnt/project-files/research/raftkv
OUT=results/verification-2026-10-11/bench
RATES=500,1000,2000,4000,8000,12000
for rep in 1 2 3 4 5; do
  for mix in 0.5 0; do
    for sys in old new etcd; do
      case $sys in
        old) args="-db raftkv -bin /tmp/bench-9667921";;
        new) args="-db raftkv -bin /tmp/bench-cfa86fa";;
        etcd) args="-db etcd -bin /tmp/bench-cfa86fa";;
      esac
      d=$OUT/r$mix/$sys/rep$rep
      mkdir -p $d
      /tmp/bench-cfa86fa/loadgen $args -read-frac $mix -rates $RATES -reps 1 -out $d > $d/stdout.txt 2>&1
      echo "$(date +%T) done rep$rep mix$mix $sys"
    done
  done
done
