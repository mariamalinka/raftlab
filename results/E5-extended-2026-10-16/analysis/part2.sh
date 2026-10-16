#!/usr/bin/env bash
# Part 2 (2026-10-16): read-path benchmarks, commit 14bf0f6, no faults, open-loop
# loadgen, 5 nodes, 10,000 keys, 128 B values, 5 s warmup, 20 s measured,
# offered 1000/2000/4000/8000 ops/s, read fraction 1.0 and 0.95.
# 5 independent repetitions = 5 fresh clusters per (path, mix); the order of the
# three paths rotates between repetitions. Each run also records: node logs
# (tail -F, since loadgen deletes its data dir), a timestamped console, and a
# 1 s CPU/RSS/PSI sample (sampler.py). Afterwards, diagnostic reruns of the two
# pilot outlier configurations with GODEBUG=gctrace=1 (labelled, not pooled).
set -u
BIN=/tmp/raftkv-bin-e5; OUT=/tmp/rk/e5x/out; SRC=/tmp/rk/raftkv-e5
DEST=/mnt/project-files/research/e5-extended-2026-10-16/raw
export TMPDIR=/tmp/rk/e5x/tmp; mkdir -p $TMPDIR
cd $SRC
bench() { d=$1; rm_=$2; rf=$3; rates=$4; shift 4
  mkdir -p $d
  L="$BIN/loadgen -bin $BIN -db raftkv -read-mode $rm_ -read-frac $rf -rates $rates -reps 1 -duration 20s -warmup 5s -keys 10000 -value-size 128 -out $d"
  echo "TMPDIR=$TMPDIR $* $L" > $d/command.txt
  rm -rf $TMPDIR/raftlab-bench
  python3 /tmp/rk/e5x/sampler.py $d/resources.csv $TMPDIR/raftlab-bench & sp=$!
  tails=""; for n in 1 2 3 4 5; do tail -F -n +1 $TMPDIR/raftlab-bench/n$n.log > $d/n$n.log 2>/dev/null & tails="$tails $!"; done
  env "$@" $L 2>&1 | while IFS= read -r line; do echo "$(date +%s.%N) $line"; done > $d/console.log
  sleep 1; kill $sp $tails 2>/dev/null; wait $sp $tails 2>/dev/null
  echo "$(date -u +%FT%TZ) bench $d" >> $OUT/attempts.log
}
echo "part2 start $(date -u +%FT%TZ) commit $(git -C $SRC rev-parse HEAD)" >> $OUT/attempts.log
paths=(log readindex lease)
for rep in 1 2 3 4 5; do
  for rf in 1.0 0.95; do
    for j in 0 1 2; do rm_=${paths[$(( (j + rep - 1) % 3 ))]}
      bench $OUT/speed/rf$rf/$rm_/rep-$rep $rm_ $rf 1000,2000,4000,8000 X=1
    done
  done
  rsync -a $OUT/ $DEST/
done
# Diagnostics for the pilot outliers (GC trace on nodes and loadgen).
for rep in 1 2 3 4 5; do
  bench $OUT/diag-gctrace/log-rf1.0-1000/rep-$rep log 1.0 1000 GODEBUG=gctrace=1
  bench $OUT/diag-gctrace/lease-rf1.0-8000/rep-$rep lease 1.0 8000 GODEBUG=gctrace=1
done
echo "part2 done $(date -u +%FT%TZ)" >> $OUT/attempts.log; rsync -a $OUT/ $DEST/; touch $OUT/PART2_DONE
