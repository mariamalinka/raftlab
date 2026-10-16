#!/usr/bin/env bash
# E4 kill-leader (2026-10-16), commit 14bf0f6: loadgen failover mode, 10 leader
# kills per system under 500 writes/s, 5 nodes, election 1 s, heartbeat 100 ms.
# Recovery = kill -> first write issued after the kill that the system
# acknowledged (Put returned OK); restarted old leader catch-up also timed.
set -u; source /tmp/rk/rq2/lib.sh; cd $SRC
echo "e4 start $(date -u +%FT%TZ)" >> $OUT/attempts.log
for db in raftkv etcd; do d=$OUT/e4-killleader/$db; mkdir -p $d
  L=($BIN/loadgen -bin $BIN -db $db -read-mode readindex -mode failover -trials 10 -failover-rate 500 -keys 10000 -value-size 128 -out $d)
  echo "TMPDIR=$TMPDIR ${L[*]}" > $d/command.txt; rm -rf $TMPDIR/raftlab-bench
  python3 /tmp/rk/rq2/sampler2.py $d/resources.csv & sp=$!
  tails=""; for n in 1 2 3 4 5; do tail -F -n +1 $TMPDIR/raftlab-bench/n$n.log > $d/n$n.log 2>/dev/null & tails="$tails $!"; done
  "${L[@]}" 2>&1 | while IFS= read -r line; do echo "$(date +%s.%N) $line"; done > $d/console.log
  sleep 1; kill $sp $tails 2>/dev/null; wait $sp $tails 2>/dev/null
done
echo "e4 done $(date -u +%FT%TZ)" >> $OUT/attempts.log; touch $OUT/E4_DONE; rsync -a $OUT/ $DEST/
