#!/usr/bin/env bash
# Runs one experiment from the research plan's matrix. Needs root.
#   scripts/experiment.sh E1            # full run (TRIALS=20)
#   TRIALS=3 TIME=20s scripts/experiment.sh E6   # pilot
set -euo pipefail
EXP=${1:?usage: experiment.sh E1..E8}
BIN=${BIN:-/tmp/raftkv-bin}
OUT=${OUT:-results}
TRIALS=${TRIALS:-20}
TIME=${TIME:-30s}
REPS=${REPS:-5}
RATES=${RATES:-500,1000,2000,4000,8000,12000}
H="$BIN/rkharness -trials $TRIALS -time $TIME -clients 25"
L="$BIN/loadgen -reps $REPS -rates $RATES"

PROCESS_FAULTS="kill-random kill-leader kill-majority pause-random pause-leader"
NET_FAULTS="partition-leader partition-halves partition-bridge"
CLOCK_FAULTS="clock-skew clock-partition clock-partition-mild"

case "$EXP" in
E1) # RQ1: linearizability of raftkv under each nemesis
  for f in $PROCESS_FAULTS $NET_FAULTS $CLOCK_FAULTS mixed; do
    $H -db raftkv -fault "$f" -out "$OUT/E1/$f"
  done
  $H -db raftkv -fault kill-all -lazyfs -out "$OUT/E1/lazyfs-kill-all"
  ;;
E2) # Harness validity: the same tests against etcd must pass
  for f in $PROCESS_FAULTS $NET_FAULTS mixed; do
    $H -db etcd -fault "$f" -out "$OUT/E2/$f"
  done
  ;;
E3) # RQ2: latency vs offered load, both systems
  for mix in 0.95 0.5 0; do for vs in 128 4096; do for dist in uniform zipf; do
    z=""; [ "$dist" = zipf ] && z="-zipf"
    for db in raftkv etcd; do
      $L -db $db -read-frac $mix -value-size $vs $z -out "$OUT/E3/$db-r$mix-v$vs-$dist"
    done
  done; done; done
  ;;
E4) # RQ2: failover gap and catch-up time
  $BIN/loadgen -db etcd -mode failover -trials "$TRIALS" -out "$OUT/E4/etcd"
  $BIN/loadgen -db raftkv -mode failover -trials "$TRIALS" -out "$OUT/E4/raftkv-snapshots"
  $BIN/loadgen -db raftkv -mode failover -trials "$TRIALS" -raftkv-args "-snapshot-bytes=0" -out "$OUT/E4/raftkv-no-snapshots"
  ;;
E5) # H1: read paths, speed and correctness, with and without clock faults
  for rm in log readindex lease; do
    $L -db raftkv -read-mode $rm -read-frac 0.95 -out "$OUT/E5/speed-$rm"
    for f in none partition-leader clock-skew clock-partition-mild clock-partition; do
      $H -db raftkv -read-mode $rm -fault "$f" -out "$OUT/E5/correct-$rm-$f"
    done
  done
  ;;
E6) # RQ3 / H2: detection rate of each injected bug under each fault
  # Unique write values let a stale read be told apart from an indeterminate
  # write. Faults run at 3, 15 and 27 s; 12 s key epochs give each fault its
  # own keys plus 4 s of healthy reads after the heal, and 34 s leaves 7 s
  # for the last fault to start and finish.
  E6H="$BIN/rkharness -trials $TRIALS -clients 25 -time ${E6_TIME:-34s} -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
  FAULTS=${FAULTS:-"kill-leader kill-majority pause-leader partition-leader partition-halves partition-bridge clock-partition-mild mixed"}
  BUGS=${BUGS:-"figure8 localread leasenodrift votenopersist notruncate nodedup ackbeforefsync"}
  for bug in $BUGS; do
    extra=""
    case $bug in
      leasenodrift) extra="-read-mode lease" ;;
      ackbeforefsync) extra="-lazyfs" ;;
    esac
    for f in $FAULTS; do
      $E6H -db raftkv -bugs $bug $extra -fault "$f" -out "$OUT/E6/$bug/$f"
    done
    [ "$bug" = ackbeforefsync ] && $E6H -db raftkv -bugs $bug $extra -fault kill-all -out "$OUT/E6/$bug/kill-all"
  done
  # Controls: the correct build under the same settings (false-positive rate
  # of the checker and the stale-read scan), on both read paths E6 uses.
  if [ "${CONTROLS:-1}" = 1 ]; then
    for f in $FAULTS; do
      $E6H -db raftkv -fault "$f" -out "$OUT/E6/correct/$f"
      $E6H -db raftkv -read-mode lease -fault "$f" -out "$OUT/E6/correct-lease/$f"
    done
    $E6H -db raftkv -lazyfs -fault kill-all -out "$OUT/E6/correct/kill-all-lazyfs"
  fi
  python3 analysis/e6_audit.py "$OUT/E6" > "$OUT/E6/AUDIT.md"
  ;;
E7) # H3: ablation of batching, pipelining and fsync against etcd
  $L -db etcd -read-frac 0.5 -out "$OUT/E7/etcd"
  $L -db etcd -read-frac 0.5 -etcd-no-fsync -out "$OUT/E7/etcd-nofsync-LABELLED-EXTRA"
  $L -db raftkv -read-frac 0.5 -out "$OUT/E7/raftkv-all-on"
  $L -db raftkv -read-frac 0.5 -raftkv-args "-batch=1" -out "$OUT/E7/raftkv-no-batching"
  $L -db raftkv -read-frac 0.5 -raftkv-args "-pipeline=false" -out "$OUT/E7/raftkv-no-pipelining"
  $L -db raftkv -read-frac 0.5 -raftkv-args "-batch=1 -pipeline=false" -out "$OUT/E7/raftkv-none"
  $L -db raftkv -read-frac 0.5 -raftkv-args "-fsync=false" -out "$OUT/E7/raftkv-nofsync-LABELLED-EXTRA"
  ;;
E8) # Context: etcd's unsafe serializable reads under partition
  for f in partition-leader partition-halves pause-leader; do
    $H -db etcd -etcd-serializable -fault "$f" -out "$OUT/E8/serializable-$f"
  done
  ;;
*) echo "unknown experiment $EXP" >&2; exit 2 ;;
esac
