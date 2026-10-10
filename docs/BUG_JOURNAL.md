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

## R-1: Follower acknowledged entries before storing them (2026-10-10)

Found by: bridge-churn fault, bug-free build
(results/faults-2026-10-10/correct/bridge-churn/trial-001).
Symptom: an acknowledged write was lost; later reads returned an older value.
Cause: HandleAppendEntries waited for durability only when the request
changed the log. A retransmission or heartbeat covering entries still queued
by an earlier request was acknowledged at once, so the leader could commit
an entry that a follower crash then erased from the majority.
Fix: wait for every queued write before replying Success (raft.go,
branch fix/follower-ack-durability). Regression tests:
internal/raft/durability_test.go. Full write-up (Polish):
docs/INVESTIGATION-lost-write.md.
Lesson: the 6.5840-style suite never crashes a node while its writes are
pending; only a fault that kills a node in the middle of replication found it.

## H-2: Lost writes never re-read, stale reads explained away (2026-10-09)

Found by: E6 pilot investigation (../E5-E6-INVESTIGATION.md).
Symptom: ackbeforefsync lost hundreds of acknowledged writes in 9/9 kill-all
faults (surviving log shorter than the acknowledged writes), yet every trial
was linearizable. localread trials came back Ok or Unknown.
Cause: two harness blind spots. Keys rotated every 5 s, as long as each fault,
so no client read a pre-fault key after recovery. And writes took one of 5
values, so a stale read could always be explained by a same-valued write left
indeterminate by the fault.
Fix: `-key-rotate` and `-unique-values` in cmd/rkharness (harness/workload.go).
With both, ackbeforefsync shows provably stale reads in 3/3 trials (0/3 for
the correct build) and localread in 2/3 under partition-leader.
Lesson: a "not detected" cell needs a reachability check: did the bug fire,
and could any client have seen it? Porcupine now times out on these
histories, so the checker is the next bottleneck.

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
