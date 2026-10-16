#!/usr/bin/env bash
# Limited E7 ablation (2026-10-16), commit 14bf0f6, existing raftkv flags only.
# 50/50 mix at 2000 and 8000 ops/s, otherwise E3 settings, 3 reps per config
# (reduced from 5 for the 2 h budget), config order rotated per rep.
#   default            : -batch 256, -pipeline, -fsync (safe)
#   batch1             : -batch=1  (batching disabled; safe)
#   nopipeline         : -pipeline=false (safe)
#   nofsync (UNSAFE)   : -fsync=false (durability weakened; not comparable as a safe config)
set -u; source /tmp/rk/rq2/lib.sh; cd $SRC
echo "e7 start $(date -u +%FT%TZ)" >> $OUT/attempts.log
names=(default batch1 nopipeline nofsync-UNSAFE); args=("" "-batch=1" "-pipeline=false" "-fsync=false")
for rep in 1 2 3; do for j in 0 1 2 3; do i=$(( (j + rep - 1) % 4 ))
  bench $OUT/e7/${names[$i]}/rep-$rep raftkv 0.5 2000,8000 "${args[$i]}"
done; done
echo "e7 done $(date -u +%FT%TZ)" >> $OUT/attempts.log; touch $OUT/E7_DONE; rsync -a $OUT/ $DEST/
