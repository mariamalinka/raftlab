#!/usr/bin/env bash
# Offline per-key Porcupine check of the Unknown lease/mild trials (60 s per call),
# then the partial add-back (TestGreedy) for any key the reduction leaves unresolved.
L=/tmp/rk/e5x/recheck/recheck.log; : > $L
for a in 001 012; do H=/tmp/rk/e5x/out/lease-mild/attempt-$a/trial-001/history.jsonl
  echo "== attempt-$a unknown_keys=$(python3 -c "import json;print(json.load(open('${H%history.jsonl}result.json'))['check'].get('unknown_keys'))")" >> $L
  for k in $(python3 -c "import json;print(' '.join(map(str,json.load(open('${H%history.jsonl}result.json'))['check'].get('unknown_keys') or [])))"); do
    RECHECK_HIST=$H RECHECK_KEYS=$k RECHECK_TIMEOUT=60s timeout 300 /tmp/rk/recheck.test -test.run TestRecheck 2>&1 | grep key >> $L
    GREEDY_HIST=$H GREEDY_KEY=$k timeout 1500 /tmp/rk/recheck.test -test.run TestGreedy 2>&1 | grep key >> $L
  done
done
echo DONE >> $L
