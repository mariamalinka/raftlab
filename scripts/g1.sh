#!/usr/bin/env bash
# Gate G1: the Raft test suite must pass 500 consecutive runs, in each
# replication mode. Usage: RUNS=500 scripts/g1.sh
set -uo pipefail
RUNS=${RUNS:-500}
OUT=${OUT:-results/G1}
mkdir -p "$OUT"
go test -c -race -o /tmp/raft.test ./internal/raft/
for mode in "" "-pipeline" "-batch=1"; do
  pass=0; fail=0
  for i in $(seq 1 "$RUNS"); do
    if /tmp/raft.test -test.count=1 $mode > "$OUT/run.log" 2>&1; then pass=$((pass+1)); else
      fail=$((fail+1)); cp "$OUT/run.log" "$OUT/fail-${mode:-default}-$i.log"; fi
  done
  echo "mode=${mode:-default} runs=$RUNS pass=$pass fail=$fail" | tee -a "$OUT/summary.txt"
done
