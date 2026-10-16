#!/usr/bin/env bash
# E3 extra repetition (rep 4) for the 50/50 and 0/100 mixes, added because the
# budget allowed 4 reps rather than 3. Same settings as e3.sh; order etcd first.
set -u; source /tmp/rk/rq2/lib.sh; cd $SRC
for rf in 0.5 0.0; do for db in etcd raftkv; do bench $OUT/e3/rf$rf/$db/rep-4 $db $rf 1000,2000,4000,8000 ""; done; done
echo "e3b done $(date -u +%FT%TZ)" >> $OUT/attempts.log; rsync -a $OUT/ $DEST/
