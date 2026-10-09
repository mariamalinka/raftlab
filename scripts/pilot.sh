#!/usr/bin/env bash
# Small first pass over E1, E2, E5 and E6 to shake out the harness before the
# full 20-trial runs. Needs root.
set -uo pipefail
BIN=${BIN:-/tmp/raftkv-bin}
OUT=${OUT:-results/pilot}
T=${TRIALS:-3}
H="$BIN/rkharness -time 20s -clients 25 -trials $T"
for f in partition-leader kill-leader partition-halves pause-leader; do
  $H -db etcd -fault $f -out $OUT/E2/$f | tail -1
done
for f in partition-leader kill-leader partition-halves pause-leader partition-bridge kill-majority clock-skew mixed; do
  $H -db raftkv -fault $f -out $OUT/E1/$f | tail -1
done
$H -db raftkv -fault kill-all -lazyfs -out $OUT/E1/lazyfs-kill-all | tail -1
for spec in "figure8 mixed" "localread partition-leader" "leasenodrift clock-partition-mild -read-mode lease" \
            "votenopersist kill-leader" "notruncate partition-halves" "nodedup partition-leader" \
            "ackbeforefsync kill-all -lazyfs"; do
  set -- $spec
  bug=$1; f=$2; shift 2
  $H -db raftkv -bugs $bug -fault $f "$@" -out $OUT/E6/$bug/$f | tail -1
done
for rm in readindex lease; do
  $H -db raftkv -read-mode $rm -fault clock-partition -out $OUT/E5/correct-$rm-clock-partition | tail -1
done
echo PILOT DONE
