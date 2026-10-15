#!/usr/bin/env bash
# G1 driver: 500 runs per mode, P concurrent runs per mode, all modes at once.
set -u
RUNS=${RUNS:-500}; P=${P:-8}; OUT=$1
mkdir -p /tmp/rk/g1logs "$OUT"
run_mode() {
  name=$1; flag=$2
  echo "mode,run,exit,seconds,race_warnings,failed_tests,start_utc" > "$OUT/runs-$name.csv"
  s=$(date +%s)
  seq 1 "$RUNS" | xargs -P "$P" -I{} bash /tmp/rk/scripts/g1-run-one.sh "$name" "$flag" {} "$OUT"
  e=$(date +%s)
  pass=$(awk -F, 'NR>1 && $3==0' "$OUT/runs-$name.csv" | wc -l)
  fail=$(awk -F, 'NR>1 && $3!=0' "$OUT/runs-$name.csv" | wc -l)
  echo "mode=$name flag='$flag' runs=$RUNS pass=$pass fail=$fail wall_seconds=$((e-s)) concurrency=$P" >> "$OUT/summary.txt"
}
run_mode default "" & run_mode pipeline "-pipeline" & run_mode batch1 "-batch=1" &
wait
echo "finished $(date -u +%FT%TZ)" >> "$OUT/summary.txt"
