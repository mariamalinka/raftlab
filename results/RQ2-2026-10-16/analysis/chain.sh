#!/usr/bin/env bash
cd /tmp/rk/rq2; until [ -f out/E3_DONE ]; do sleep 5; done
./overload.sh > overload.log 2>&1; ./e7.sh > e7.log 2>&1; ./e4.sh > e4.log 2>&1; touch out/ALL_DONE
