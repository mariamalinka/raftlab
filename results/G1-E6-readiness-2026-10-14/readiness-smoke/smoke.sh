#!/usr/bin/env bash
# E6 readiness smoke: 3 trials per cell with the proposed E6 settings.
set -u
BIN=/tmp/raftkv-bin; OUT=${OUT:-/tmp/rk/smoke}; T=${TRIALS:-3}
H="$BIN/rkharness -bin $BIN -trials $T -clients 25 -time 34s -warmup 3s -interval 7s -fault-duration 5s -recover 5s -key-rotate 12s -unique-values"
run() { name=$1; shift; echo "== $name $(date -u +%T)"; $H "$@" -out "$OUT/$name" 2>&1 | tail -4; }
cd /tmp/rk/raftkv
run ackbeforefsync/kill-all       -db raftkv -bugs ackbeforefsync -lazyfs -fault kill-all
run ackbeforefsync/kill-majority  -db raftkv -bugs ackbeforefsync -lazyfs -fault kill-majority
run correct/kill-all-lazyfs       -db raftkv -lazyfs -fault kill-all
run localread/partition-leader    -db raftkv -bugs localread -fault partition-leader
run correct/partition-leader      -db raftkv -fault partition-leader
run leasenodrift/clock-partition-mild -db raftkv -bugs leasenodrift -read-mode lease -fault clock-partition-mild
run correct-lease/clock-partition-mild -db raftkv -read-mode lease -fault clock-partition-mild
run nodedup/pause-leader          -db raftkv -bugs nodedup -fault pause-leader
run votenopersist/kill-majority   -db raftkv -bugs votenopersist -fault kill-majority
run notruncate/partition-halves   -db raftkv -bugs notruncate -fault partition-halves
run figure8/mixed                 -db raftkv -bugs figure8 -fault mixed
echo "done $(date -u +%T)"
