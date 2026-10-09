# Bug journal

Every real bug found in our own code (store or harness), the evidence that
exposed it, and the fix. Kept from day one; these stories feed the report.
Entries are newest first. "R-" entries are bugs in raftkv, "H-" entries are
bugs or blind spots in the harness.

Template:

```
## R-n: one-line title (date)
Found by: test / fault / trial folder
Symptom: what the checker or test reported
Cause: what was actually wrong
Fix: commit or file:line
Lesson: what it says about the method
```

---

## H-1: Leader-chasing clients hide stale reads (2026-10-09)

Found by: pilot of E6, bug `localread` under `partition-leader`
(scratch runs, not archived).

Symptom: with the local-read bug switched on, 4 of 4 trials came back
linearizable with 10 clients. The bug serves reads on a deposed leader, so
the harness should have caught it.

Cause: the client library follows leader hints, so every client moved to the
new leader within one request. During the ~700 ms window in which the old
leader still believed it led (check-quorum had not fired yet), almost no
client was reading from it. Clients pinned to the old node were mostly stuck
in a write that could not commit, waiting for the 500 ms attempt timeout.

Fix: bind each harness client to a home node, as Jepsen does
(`client.Client.Home`, `harness/db.go` NewClient), and raise the default to
25 clients (5 per node). Detection went from 0/4 to 2/3 trials, both caught
21.1 s into the run.

Lesson: detection rate depends on the workload as much as on the fault. The
E6 results must report client count, home-node binding and op mix, and a
"not detected" cell may say more about the workload than about the bug.
