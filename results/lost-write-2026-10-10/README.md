# Evidence for docs/INVESTIGATION-lost-write.md

- regression-test-before-fix.txt / regression-test-after-fix.txt: output of the two new tests on commit 9667921 and with the fix.
- instrumented-bridge-churn/: 3 bridge-churn trials with a scratch build of 9667921 that logs "INSTR ack beyond durable" (not in the repository).
- before-fix/bridge-churn-20/: 20 trials, unfixed build (binary built from 9667921's raft code).
- after-fix/: 20 bridge-churn trials, E1 rechecks and the ackbeforefsync check, fixed build.
- unit-suites-after-fix.txt: vet, Raft suite in three modes, all packages, make bugtests.
The original failing trial is untouched in results/faults-2026-10-10/correct/bridge-churn/trial-001.
