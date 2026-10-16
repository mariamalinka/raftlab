#!/usr/bin/env bash
# E5 pilot (2026-10-16), commit 14bf0f6. Correctness side: 3 read paths x 3 clock
# conditions x 3 trials, one trial per rkharness process, E6H settings (5 nodes,
# 25 clients, 34 s, faults at 3/15/27 s for 5 s, 12 s key epochs, unique values).
# Conditions use the same leader isolation; only the leader's clock rate differs:
#   normal = partition-leader (clock x1.00)
#   mild   = clock-partition-mild (leader clock x0.92, inside the 10% drift bound)
#   severe = clock-partition (leader clock x0.25, outside the bound)
# Speed side: open-loop loadgen, no faults, read-only and 95% reads, same offered loads per path.
set -u
BIN=/tmp/raftkv-bin-e5; OUT=/tmp/rk/e5/out; SRC=/tmp/rk/raftkv-e5
DEST=/mnt/project-files/research/e5-pilot-2026-10-16/raw
H="$BIN/rkharness -bin $BIN -trials 1 -clients 25 -time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
export TMPDIR=/tmp/rk/e5/tmp; mkdir -p $TMPDIR $OUT
cd $SRC
one() { cell=$1; i=$2; shift 2
  d=$OUT/correctness/$cell/attempt-$(printf %03d $i)
  [ -f $d/trial-001/result.json ] && return 0
  rm -rf $d; mkdir -p $d; echo "TMPDIR=$TMPDIR $H $* -out $d" > $d/command.txt
  t0=$(date +%s); $H "$@" -out "$d" > $d/console.log 2>&1 & pid=$!; peak=0
  while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 1; done
  wait $pid; rc=$?; note=""
  if [ ! -f $d/trial-001/result.json ]; then note="NO_RESULT"
    cp $TMPDIR/raftlab-data/n*.log $d/trial-001/ 2>/dev/null && note="$note node-logs-recovered"
    dmesg 2>/dev/null | grep -i -E "oom-kill|Killed process" | tail -3 > $d/dmesg-tail.txt
    pkill -9 -f "$BIN/raftkv"; sleep 2; fi
  echo "$(date -u +%FT%TZ) $cell attempt=$i exit=$rc wall_s=$(( $(date +%s)-t0 )) peak_rss_kb=$peak $note | $(grep '^trial' $d/console.log | tail -1 | cut -c1-170)" | tee -a $OUT/attempts.log
  rsync -a $OUT/ $DEST/; }
echo "start $(date -u +%FT%TZ) commit $(git -C $SRC rev-parse HEAD)" >> $OUT/attempts.log
for rm in log readindex lease; do
  for c in "normal partition-leader" "mild clock-partition-mild" "severe clock-partition"; do
    set -- $c
    for i in 1 2 3; do one $rm/$1 $i -db raftkv -read-mode $rm -fault $2; done
  done
done
echo "correctness done $(date -u +%FT%TZ)" >> $OUT/attempts.log
# Speed: identical open-loop offered loads for every path, no faults.
for rf in 1.0 0.95; do for rm in log readindex lease; do
  p=$OUT/speed/rf$rf/$rm; mkdir -p $p
  L="$BIN/loadgen -bin $BIN -db raftkv -read-mode $rm -read-frac $rf -rates 1000,4000,8000 -reps 2 -duration 20s -warmup 5s -out $p"
  echo "TMPDIR=$TMPDIR $L" > $p/command.txt
  $L > $p/console.log 2>&1; echo "$(date -u +%FT%TZ) speed rf=$rf $rm exit=$?" >> $OUT/attempts.log
  rsync -a $OUT/ $DEST/
done; done
echo "done $(date -u +%FT%TZ)" >> $OUT/attempts.log; rsync -a $OUT/ $DEST/; touch $OUT/DONE
