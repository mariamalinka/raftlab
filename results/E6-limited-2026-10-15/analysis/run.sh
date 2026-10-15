#!/usr/bin/env bash
# Limited E6 (2026-10-15): ackbeforefsync and localread, 20 trials each, each
# with a bug-free control under the identical configuration.
# Flags are the E6H line of scripts/experiment.sh at commit 85a3e22.
set -u
BIN=/tmp/raftkv-bin-e6l; OUT=/tmp/rk/e6l/out; TRIALS=20
E6H="$BIN/rkharness -bin $BIN -trials $TRIALS -clients 25 -time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
cd /tmp/rk/e6l/raftkv
run() { name=$1; shift; echo "== $name start $(date -u +%FT%TZ)"; echo "$E6H $* -out $OUT/E6/$name" > /tmp/rk/e6l/cmd-${name//\//_}.txt; $E6H "$@" -out "$OUT/E6/$name"; echo "== $name end $(date -u +%FT%TZ)"; rsync -a $OUT/ /mnt/project-files/research/e6-limited-2026-10-15/raw/; }
run ackbeforefsync/kill-all  -db raftkv -bugs ackbeforefsync -lazyfs -fault kill-all
run correct-lazyfs/kill-all  -db raftkv -lazyfs -fault kill-all
run localread/partition-leader -db raftkv -bugs localread -fault partition-leader
run correct/partition-leader -db raftkv -fault partition-leader
echo "done $(date -u +%FT%TZ)"
