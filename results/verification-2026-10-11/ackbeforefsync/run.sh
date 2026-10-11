#!/bin/bash
# ackbeforefsync detection: old vs new raftkv-bugs, same (current) rkharness,
# same settings as the earlier runs (kill-all, lazyfs, unique values, key-rotate 15s,
# 25 clients). TIME=20s reproduces the after-fix run (2 kills fit); TIME=25s lets
# the third kill fit. Builds alternate in blocks of 5 trials.
set -u
cd /mnt/project-files/research/raftkv
OUT=results/verification-2026-10-11/ackbeforefsync
for time in 20s 25s; do
  for blk in 1 2 3 4; do
    for build in old new; do
      bin=/tmp/raftkv-bin; [ $build = new ] && bin=/tmp/raftkv-bin-fix
      /tmp/raftkv-bin/rkharness -bin $bin -db raftkv -bugs ackbeforefsync -fault kill-all -lazyfs \
        -unique-values -key-rotate 15s -clients 25 -time $time -trials 5 \
        -out $OUT/$build-$time/block$blk > $OUT/$build-$time-block$blk.log 2>&1
      echo "$(date +%T) done $build $time block$blk"
    done
  done
done
