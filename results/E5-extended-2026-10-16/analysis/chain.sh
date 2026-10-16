#!/usr/bin/env bash
while [ ! -e /tmp/rk/e5x/out/PART1_DONE ]; do sleep 20; done
[ -e /tmp/rk/e5x/out/STOP ] && { echo "part2 skipped: safety stop" >> /tmp/rk/e5x/out/attempts.log; exit 0; }
bash /tmp/rk/e5x/part2.sh > /tmp/rk/e5x/part2.log 2>&1
