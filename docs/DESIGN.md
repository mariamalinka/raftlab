# Design notes

Decisions that matter for interpreting results. Each one names where it lives.

## Raft (`internal/raft/raft.go`)

- **One mutex, one persister goroutine.** All disk writes are queued in log
  order and written by a single goroutine, which groups everything queued
  into one fsync (group commit). RPC handlers that must not reply before
  their writes are durable (votes, AppendEntries) wait on the persister. The
  leader counts itself towards a commit quorum only for entries it has made
  durable (`selfMatch`).
- **Batching** (`MaxBatch`): caps entries per AppendEntries and operations per
  fsync group. `-batch=1` disables both, which is the E7 ablation.
- **Pipelining** (`Pipeline`, `MaxInflight`): the leader advances `nextIndex`
  optimistically and keeps up to 8 AppendEntries in flight per follower. On a
  lost message it rewinds to `matchIndex+1`.
- **Leader no-op** at the start of each term, so ReadIndex and leases can
  require a committed entry from the current term (thesis §6.4).
- **Fast log backtracking** with conflict term/index hints (paper §5.3).
- **Check-quorum:** a leader that hears from no majority within an election
  timeout steps down.
- **Vote stickiness** (thesis §4.2.3, §6.4.1): a node that heard from a leader
  within the minimum election timeout refuses to vote for a higher term. A
  freshly restarted node refuses for one timeout after boot, because it may
  have acknowledged a lease just before crashing. Both are required for lease
  safety. Note for E6: the boot embargo does not mask `votenopersist`,
  because a second candidate in the *same* term passes the term check.

## Reads (`internal/kv/store.go`)

- `log`: Get goes through the log like a write.
- `readindex`: record commit index, confirm leadership with one round of
  AppendEntries acknowledged by a majority, wait until applied, read.
- `lease`: if a majority acknowledged messages sent within
  `ElectionTimeout × (1 − LeaseDriftBound)` (leader clock, measured from send
  time), read at the commit index without a round; otherwise ReadIndex.

## Clients and sessions

- Every write carries (client ID, sequence number). The state machine keeps
  the last sequence and result per client and returns the cached result for
  a duplicate. Reads are not deduplicated.
- A write whose outcome is unknown (timeout, connection error, or leader
  change after proposing) is reported as indeterminate and recorded as
  `info`, never as a failure.
- Harness clients are bound to a home node, like Jepsen processes; followers
  answer `not_leader` with a hint. Unbound clients all chase the leader and
  almost never read from a deposed one, which hid stale reads in the first
  pilot (see the bug journal, H-1).

## Storage (`internal/wal/wal.go`)

Record: `crc32c | length | type | payload`; types HardState, Entry, Truncate.
Recovery replays until the first bad checksum and truncates the tail.
Snapshots are written to a temp file, fsynced, renamed, and the log is
rewritten without the compacted prefix.

## Checker (`harness/checker.go`)

Porcupine with a nondeterministic register model partitioned by key.
Indeterminate writes and CASes may or may not take effect; indeterminate
reads are dropped. On an anomaly, the checker narrows to failing keys,
writes Porcupine's HTML visualization, and binary-searches the shortest
failing history prefix (100 ms resolution) for time-to-detection.

## Defaults matched to etcd

Election timeout 1 s, heartbeat 100 ms (etcd defaults), 5 nodes, fsync on.
etcd always batches and pipelines; raftkv's defaults match that, and E7
switches them off one at a time.
