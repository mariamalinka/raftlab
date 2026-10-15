#!/usr/bin/env bash
# Resume of the limited E6 after the first rkharness process for each bug cell
# was OOM-killed while checking a trial (2026-10-15). Same flags; one trial per
# process, so a checker crash loses only that trial. Peak RSS is sampled.
set -u
BIN=/tmp/raftkv-bin-e6l; OUT=/tmp/rk/e6l/out
E6H="$BIN/rkharness -bin $BIN -trials 1 -clients 25 -time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
cd /tmp/rk/e6l/raftkv
one() { cell=$1; i=$2; shift 2; d=$OUT/E6/$cell-single/attempt-$(printf %03d $i); mkdir -p $d
  echo "$E6H $* -out $d" > $d/command.txt
  $E6H "$@" -out "$d" > $d/console.log 2>&1 & pid=$!; peak=0
  while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 1; done
  wait $pid; rc=$?; echo "$cell attempt=$i exit=$rc peak_rss_kb=$peak $(tail -1 $d/console.log | cut -c1-150)" | tee -a $OUT/E6/attempts.log
  rsync -a $OUT/ /mnt/project-files/research/e6-limited-2026-10-15/raw/; }
for i in $(seq 5 20); do one ackbeforefsync/kill-all $i -db raftkv -bugs ackbeforefsync -lazyfs -fault kill-all; done
for i in $(seq 9 20); do one localread/partition-leader $i -db raftkv -bugs localread -fault partition-leader; done
echo "done $(date -u +%FT%TZ)" >> $OUT/E6/attempts.log
