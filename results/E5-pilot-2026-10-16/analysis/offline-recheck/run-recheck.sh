#!/usr/bin/env bash
# Offline per-key Porcupine check of the 3 Unknown lease/severe trials: full per-key
# check (60 s), then the harness's reduction if Unknown. One key per process.
O=/tmp/rk/e5/out/correctness/lease/severe; L=/tmp/rk/e5/recheck/recheck.log
for a in 001 002 003; do
  for k in $(python3 -c "import json;print(' '.join(str(k) for k in sorted({json.loads(l)['key'] for l in open('$O/attempt-$a/trial-001/history.jsonl')})))"); do
    echo "== attempt-$a key=$k" >> $L
    RECHECK_HIST=$O/attempt-$a/trial-001/history.jsonl RECHECK_KEYS=$k RECHECK_TIMEOUT=60s /tmp/rk/recheck.test -test.run TestRecheck -test.timeout 1h >> $L 2>&1 & pid=$!; peak=0
    while kill -0 $pid 2>/dev/null; do r=$(awk '/VmRSS/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && [ "$r" -gt "$peak" ] && peak=$r; sleep 1; done
    wait $pid; echo "exit=$? peak_rss_kb=$peak" >> $L
  done
done
echo DONE >> $L
