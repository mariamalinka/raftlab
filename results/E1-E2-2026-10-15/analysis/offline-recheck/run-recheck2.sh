#!/usr/bin/env bash
# Second pass: reduced check only (as the harness does after Unknown), one key per process, 20 min.
O=/tmp/rk/e1e2/out; L=/tmp/rk/e1e2/recheck/recheck.log
rc() { echo "== pass2 reduced-only $1 key=$2 $(date -u +%FT%TZ)" >> $L
  RECHECK_MODE=reduced RECHECK_HIST=$O/$1/trial-001/history.jsonl RECHECK_KEYS=$2 RECHECK_TIMEOUT=20m /tmp/rk/recheck.test -test.run TestRecheck -test.timeout 10h >> $L 2>&1 & pid=$!; peak=0
  while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 2; done
  wait $pid; echo "exit=$? peak_rss_kb=$peak" >> $L; }
rc E2/partition-leader/attempt-008 4
rc E2/mixed/attempt-011 3
for k in $(python3 -c "import json;print(' '.join(str(k) for k in sorted({json.loads(l)['key'] for l in open('$O/E2/partition-leader/attempt-005/trial-001/history.jsonl')})))"); do rc E2/partition-leader/attempt-005 $k; done
echo DONE2 >> $L
