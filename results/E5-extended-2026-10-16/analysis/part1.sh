#!/usr/bin/env bash
# Part 1 (2026-10-16): 60 lease x clock-partition-mild (leader clock 0.92x) trials,
# commit 14bf0f6, identical flags to the E5 pilot, one trial per rkharness process.
# Stops at the first trial with Porcupine Illegal or a provably stale read.
set -u
BIN=/tmp/raftkv-bin-e5; OUT=/tmp/rk/e5x/out; SRC=/tmp/rk/raftkv-e5
DEST=/mnt/project-files/research/e5-extended-2026-10-16/raw
H="$BIN/rkharness -bin $BIN -trials 1 -clients 25 -time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
export TMPDIR=/tmp/rk/e5x/tmp; mkdir -p $TMPDIR $OUT
cd $SRC
echo "start $(date -u +%FT%TZ) commit $(git -C $SRC rev-parse HEAD)" >> $OUT/attempts.log
for i in $(seq 1 60); do
  [ -e $OUT/STOP ] && break
  d=$OUT/lease-mild/attempt-$(printf %03d $i)
  [ -f $d/trial-001/result.json ] && continue
  rm -rf $d; mkdir -p $d; echo "TMPDIR=$TMPDIR $H -db raftkv -read-mode lease -fault clock-partition-mild -out $d" > $d/command.txt
  t0=$(date +%s); $H -db raftkv -read-mode lease -fault clock-partition-mild -out "$d" > $d/console.log 2>&1 & pid=$!; peak=0
  while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 1; done
  wait $pid; rc=$?; note=""
  if [ ! -f $d/trial-001/result.json ]; then note="NO_RESULT"
    cp $TMPDIR/raftlab-data/n*.log $d/trial-001/ 2>/dev/null && note="$note node-logs-recovered"
    dmesg 2>/dev/null | grep -i -E "oom-kill|Killed process" | tail -3 > $d/dmesg-tail.txt
    pkill -9 -f "$BIN/raftkv"; sleep 2
    python3 /tmp/rk/e1e2/stale_offline.py $d/trial-001/history.jsonl > $d/offline-stale.txt 2>&1 || { echo "$d" > $OUT/STOP; note="$note OFFLINE-STALE"; }
  else
    python3 -c "import json,sys;r=json.load(open('$d/trial-001/result.json'));sys.exit(1 if r['check']['result']=='Illegal' or (r.get('stale_reads') or {}).get('reads',0)>0 else 0)" || { echo "$d" > $OUT/STOP; note="$note SAFETY-STOP"; }
  fi
  echo "$(date -u +%FT%TZ) lease-mild attempt=$i exit=$rc wall_s=$(( $(date +%s)-t0 )) peak_rss_kb=$peak $note | $(grep '^trial' $d/console.log | tail -1 | cut -c1-170)" | tee -a $OUT/attempts.log
  rsync -a $OUT/ $DEST/
done
echo "part1 done $(date -u +%FT%TZ)" >> $OUT/attempts.log; rsync -a $OUT/ $DEST/; touch $OUT/PART1_DONE
