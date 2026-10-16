#!/usr/bin/env bash
# E3 (2026-10-16): RaftKV (ReadIndex) vs etcd 3.7.2 (linearizable reads), commit 14bf0f6.
# 5 nodes, open loop, 10k keys, 128 B, 5 s warmup, 20 s window, 1000-8000 ops/s.
# 95/5: etcd 5 fresh reps; RaftKV 95/5 reuses the 5 E5-extension readindex reps
# (same commit, container, method) plus 2 fresh cross-check reps here.
# 50/50 and 0/100: 3 reps per system (reduced from 5 for the 2 h budget).
# Each rep = fresh cluster; system order alternates between reps.
set -u; source /tmp/rk/rq2/lib.sh; mkdir -p $OUT; cd $SRC
echo "e3 start $(date -u +%FT%TZ) commit $(git rev-parse HEAD)" >> $OUT/attempts.log
R=1000,2000,4000,8000
for rep in 1 2 3 4 5; do
  bench $OUT/e3/rf0.95/etcd/rep-$rep etcd 0.95 $R ""
  [ $rep -le 2 ] && bench $OUT/e3/rf0.95/raftkv/rep-$rep raftkv 0.95 $R ""
done
for rf in 0.5 0.0; do for rep in 1 2 3; do
  if [ $((rep % 2)) = 1 ]; then order="raftkv etcd"; else order="etcd raftkv"; fi
  for db in $order; do bench $OUT/e3/rf$rf/$db/rep-$rep $db $rf $R ""; done
done; done
echo "e3 done $(date -u +%FT%TZ)" >> $OUT/attempts.log; touch $OUT/E3_DONE
mkdir -p $DEST; rsync -a $OUT/ $DEST/
