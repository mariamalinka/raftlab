#!/usr/bin/env bash
# E1 (RaftKV, bug-free build) and E2 (etcd v3.7.2 control), 2026-10-15.
# Commit 80f06fc (GitHub main). Settings are the E6H line of
# scripts/experiment.sh (the limited E6 methodology): 5 nodes, 25 clients,
# 34 s workload, faults at 3/15/27 s lasting 5 s, 5 s recovery, 12 s key
# epochs, unique write values, ReadIndex reads (raftkv) / linearizable reads (etcd).
#
# One trial per rkharness process (-trials 1), so a checker crash costs one
# trial. history.jsonl and nemesis.jsonl are written by the harness before
# Porcupine runs; if the process dies while checking, node logs are copied
# here from its data dir (they are otherwise only saved after checking).
# Peak RSS is sampled. A trial with Illegal or a provably stale read stops
# the whole run (STOP file) so its evidence can be investigated first.
set -u
BIN=/tmp/raftkv-bin-e1; OUT=/tmp/rk/e1e2/out; SRC=/tmp/rk/raftkv-e1
DEST=/mnt/project-files/research/e1-e2-2026-10-15/raw
TRIALS=${TRIALS:-20}
H="$BIN/rkharness -bin $BIN -trials 1 -clients 25 -time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
export TMPDIR=/tmp/rk/e1e2/tmp; mkdir -p $TMPDIR $OUT
cd $SRC
one() { exp=$1; cell=$2; i=$3; shift 3
  [ -e $OUT/STOP ] && return 1
  d=$OUT/$exp/$cell/attempt-$(printf %03d $i)
  if [ -f $d/trial-001/result.json ]; then return 0; fi   # resume: keep finished trials
  rm -rf $d; mkdir -p $d
  echo "TMPDIR=$TMPDIR $H $* -out $d" > $d/command.txt
  t0=$(date +%s)
  $H "$@" -out "$d" > $d/console.log 2>&1 & pid=$!; peak=0
  while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 1; done
  wait $pid; rc=$?
  note=""
  if [ ! -f $d/trial-001/result.json ]; then
    note="NO_RESULT"
    cp $TMPDIR/raftlab-data/n*.log $d/trial-001/ 2>/dev/null && note="$note node-logs-recovered"
    dmesg 2>/dev/null | grep -i -E "out of memory|oom-kill|Killed process" | tail -3 > $d/dmesg-tail.txt
    pkill -9 -f "$BIN/raftkv" ; pkill -9 -f "$BIN/etcd"; sleep 2
  fi
  echo "$(date -u +%FT%TZ) $exp/$cell attempt=$i exit=$rc wall_s=$(( $(date +%s)-t0 )) peak_rss_kb=$peak $note | $(grep '^trial' $d/console.log | tail -1 | cut -c1-160)" | tee -a $OUT/attempts.log
  rsync -a $OUT/ $DEST/
  if [ -f $d/trial-001/result.json ]; then
    python3 - "$d/trial-001/result.json" <<'PY' || { echo "SAFETY STOP: $d" | tee -a $OUT/attempts.log; echo "$d" > $OUT/STOP; rsync -a $OUT/ $DEST/; return 1; }
import json,sys
r=json.load(open(sys.argv[1]))
sys.exit(1 if r["check"]["result"]=="Illegal" or (r.get("stale_reads") or {}).get("reads",0)>0 else 0)
PY
  else
    python3 /tmp/rk/e1e2/stale_offline.py $d/trial-001/history.jsonl > $d/offline-stale.txt 2>&1 || { echo "SAFETY STOP (offline stale read in crashed trial): $d" | tee -a $OUT/attempts.log; echo "$d" > $OUT/STOP; rsync -a $OUT/ $DEST/; return 1; }
  fi
}
cell() { exp=$1; c=$2; shift 2; for i in $(seq 1 $TRIALS); do one $exp $c $i "$@" || return 1; done; }
run_all() {
  # E1: RaftKV, correct build, ReadIndex
  for f in kill-leader kill-majority pause-leader partition-leader partition-halves partition-bridge clock-skew mixed; do
    cell E1 $f -db raftkv -fault $f || return 1
  done
  cell E1 kill-all-lazyfs -db raftkv -lazyfs -fault kill-all || return 1
  # E2: etcd, linearizable reads. clock-skew is not supported for etcd (no
  # in-process clock); kill-all runs without LazyFS (raftkv-only WAL mode).
  for f in kill-leader kill-majority pause-leader partition-leader partition-halves partition-bridge mixed kill-all; do
    cell E2 $f -db etcd -fault $f || return 1
  done
}
echo "start $(date -u +%FT%TZ) commit $(git -C $SRC rev-parse HEAD)" >> $OUT/attempts.log
run_all
echo "done $(date -u +%FT%TZ)" >> $OUT/attempts.log
rsync -a $OUT/ $DEST/
touch $OUT/DONE
