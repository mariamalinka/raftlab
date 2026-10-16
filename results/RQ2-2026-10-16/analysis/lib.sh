BIN=/tmp/raftkv-bin-e5; OUT=/tmp/rk/rq2/out; SRC=/tmp/rk/raftkv-e5
DEST=/mnt/project-files/research/rq2-2026-10-16/raw
export TMPDIR=/tmp/rk/rq2/tmp
# bench DIR DB RF RATES "RAFTKV_ARGS" [ENV...]
bench() { d=$1; db=$2; rf=$3; rates=$4; rargs=$5; shift 5
  mkdir -p $d
  L=($BIN/loadgen -bin $BIN -db $db -read-mode readindex -read-frac $rf -rates $rates -reps 1 -duration 20s -warmup 5s -keys 10000 -value-size 128 -out $d)
  [ -n "$rargs" ] && L+=(-raftkv-args "$rargs")
  echo "TMPDIR=$TMPDIR $* ${L[*]}" > $d/command.txt
  rm -rf $TMPDIR/raftlab-bench
  python3 /tmp/rk/rq2/sampler2.py $d/resources.csv & sp=$!
  tails=""; for n in 1 2 3 4 5; do tail -F -n +1 $TMPDIR/raftlab-bench/n$n.log > $d/n$n.log 2>/dev/null & tails="$tails $!"; done
  env X=1 "$@" "${L[@]}" 2>&1 | while IFS= read -r line; do echo "$(date +%s.%N) $line"; done > $d/console.log
  sleep 1; kill $sp $tails 2>/dev/null; wait $sp $tails 2>/dev/null
  echo "$(date -u +%FT%TZ) bench $d" >> $OUT/attempts.log
}
