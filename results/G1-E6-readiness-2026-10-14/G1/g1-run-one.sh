#!/usr/bin/env bash
# One G1 run: g1-run-one.sh <mode-name> <mode-flag-or-empty> <run-index> <outdir>
# No retries: the exit code of the single run is recorded as-is.
name=$1; flag=$2; i=$3; out=$4
log=$(mktemp /tmp/rk/g1logs/$name-$i.XXXX)
s=$(date +%s.%N)
/tmp/rk/raft.test -test.count=1 -test.v $flag > "$log" 2>&1; rc=$?
e=$(date +%s.%N)
dur=$(echo "$e - $s" | bc)
failed=$(grep -- '^--- FAIL' "$log" | awk '{print $3}' | tr '\n' ' ')
race=$(grep -c 'WARNING: DATA RACE' "$log")
echo "$name,$i,$rc,$dur,$race,\"$failed\",$(date -u -d @${s%.*} +%FT%TZ)" >> "$out/runs-$name.csv"
if [ "$rc" != 0 ]; then cp "$log" "$out/fail-$name-$i.log"; fi
rm -f "$log"
