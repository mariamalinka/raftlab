#!/usr/bin/env bash
# Offline per-key re-check of the 4 inconclusive E2 trials, 20 min per check.
O=/tmp/rk/e1e2/out; L=/tmp/rk/e1e2/recheck/recheck.log
rc() { echo "== $1 keys=$2 $(date -u +%FT%TZ)" >> $L
  RECHECK_HIST=$O/$1/trial-001/history.jsonl RECHECK_KEYS=$2 RECHECK_TIMEOUT=20m /tmp/rk/recheck.test -test.run TestRecheck -test.timeout 10h >> $L 2>&1 & pid=$!; peak=0
  while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 2; done
  wait $pid; echo "exit=$? peak_rss_kb=$peak" >> $L; }
rc E2/partition-leader/attempt-008 4
rc E2/partition-leader/attempt-019 2
rc E2/mixed/attempt-011 3
rc E2/partition-leader/attempt-005 all
echo DONE >> $L
