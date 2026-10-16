#!/usr/bin/env bash
# Overload diagnostics (2026-10-16), commit 14bf0f6. 50/50 mix (where the
# 2026-10-11 collapse was seen), 8000-14000 ops/s, otherwise E3 settings.
# RaftKV 2 reps, etcd 1 rep (control: same loadgen, same host).
# Then one labelled diagnostic RaftKV run at 12000 ops/s: ~20 s after the
# cluster is up, SIGQUIT (GOTRACEBACK=all) to the leader and one follower to
# capture goroutine dumps. That run's latency numbers are not used.
set -u; source /tmp/rk/rq2/lib.sh; cd $SRC
echo "overload start $(date -u +%FT%TZ)" >> $OUT/attempts.log
R=8000,10000,12000,14000
bench $OUT/overload/raftkv/rep-1 raftkv 0.5 $R ""
bench $OUT/overload/etcd/rep-1 etcd 0.5 $R ""
bench $OUT/overload/raftkv/rep-2 raftkv 0.5 $R ""
# Added 18:12Z: E3 RaftKV write-only rep 3 collapsed at 8000 ops/s (3949 achieved,
# 40k errors + 40k drops); one write-only RaftKV run around that point.
bench $OUT/overload/raftkv-writeonly/rep-1 raftkv 0.0 6000,8000,10000 ""
d=$OUT/overload/diag-goroutines-12000; mkdir -p $d
( while [ ! -f $d/config.json ]; do sleep 0.2; done; sleep 20
  L=$(grep -l '"became leader"' $TMPDIR/raftlab-bench/n*.log | tail -1); id=$(basename $L .log | tr -d n)
  f=$(( id % 5 + 1 ))
  date -u +%FT%T.%NZ > $d/sigquit.txt; echo "leader=n$id follower=n$f" >> $d/sigquit.txt
  pgrep -a -x raftkv >> $d/sigquit.txt
  for n in $id $f; do pid=$(pgrep -f "raftkv .*-id $n( |$)" | head -1); echo "n$n pid=$pid" >> $d/sigquit.txt; kill -QUIT $pid; done ) &
bench $d raftkv 0.5 12000 "" GOTRACEBACK=all
wait
echo "overload done $(date -u +%FT%TZ)" >> $OUT/attempts.log; touch $OUT/OVERLOAD_DONE; rsync -a $OUT/ $DEST/
