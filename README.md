# raftkv: a from-scratch Raft key-value store, tested against etcd

Research question: can a from-scratch Raft KV store stay linearizable under
crashes, partitions, pauses and clock skew, what does each correctness
mechanism cost compared with etcd, and how good is a Jepsen-style harness at
finding real Raft bugs? The full plan is in
`../../Raft KV Store vs etcd — Research Plan.pdf`.

## Layout

| Path | What it is |
|---|---|
| `internal/raft` | Raft core: election, replication, commit, snapshots, ReadIndex, leases, batching, pipelining. Tests follow the MIT 6.5840 lab structure (gate G1). |
| `internal/wal` | Append-only write-ahead log, CRC-32C per record, torn-tail recovery, snapshots, fsync policy, LazyFS stand-in. |
| `internal/kv` | Replicated KV state machine: Put, CAS, Get over three read paths (`log`, `readindex`, `lease`), client-session dedup. |
| `internal/bugs` | The seven injectable bugs. Constant `false` in normal builds (compiled out); enabled with `-tags bugs` + `RAFTKV_BUGS`. |
| `internal/clock` | Skewable clock (jumps and rate changes over the admin API), used instead of libfaketime. |
| `internal/rpc`, `internal/server` | gRPC transport with a small binary codec; node wiring. |
| `client` | Client library: leader discovery, retries, client ID + sequence numbers, indeterminate results. |
| `cmd/raftkv` | Node binary. |
| `harness`, `cmd/rkharness` | Fault-injection harness: one network namespace per node, kill/pause/iptables/clock nemeses, register workload, Porcupine checker, Wilson intervals. |
| `bench`, `cmd/loadgen` | Open-loop load generator with HdrHistogram, raftkv and etcd backends; failover mode for E4. |
| `tools/etcd` | Builds the pinned etcd release (v3.7.2) used as the control. |
| `scripts/experiment.sh` | One entry per experiment E1–E8; `scripts/g1.sh` is gate G1. |
| `docs/` | Bug journal, experiment log, design notes, threats to validity. |
| `results/` | Raw histories, checker verdicts, benchmark logs and configs, one folder per run. |

## Running

Needs Linux, root (network namespaces and iptables), `iproute2`, Go 1.26.

```sh
make build            # binaries into /tmp/raftkv-bin (override with BIN=)
make test             # unit tests with the race detector
make g1               # Raft suite 500x in each replication mode
make e1               # or e2 ... e8; TRIALS=3 TIME=20s for a quick pass
make report
```

A single test by hand:

```sh
/tmp/raftkv-bin/rkharness -db raftkv -fault partition-leader -trials 20 -clients 25 -out results/E1/partition-leader
/tmp/raftkv-bin/rkharness -db raftkv -bugs localread -fault partition-leader -trials 20 -clients 25 -out results/E6/localread/partition-leader
/tmp/raftkv-bin/loadgen -db etcd -rates 1000,2000,4000 -read-frac 0.95 -out results/E3/etcd-95
```

Each trial folder holds `history.jsonl` (every operation), `nemesis.jsonl`
(every fault), node logs (JSON, tagged with node, term and index),
`result.json`, and `linearizability.html` (Porcupine's visualization) when an
anomaly is found. `summary.json` aggregates a configuration.

## Deviations from the plan

- **Harness: Go + Porcupine, not Jepsen.** The plan recommends Jepsen and names
  this as the fallback. It was chosen here because it runs in one container
  with no SSH nodes. Faults are still real (kill -9, SIGSTOP, iptables in
  separate network namespaces). Porting the workload to Jepsen later is a
  credibility upgrade, not a prerequisite.
- **Clock skew via an in-process skewable clock**, not libfaketime. It only
  affects raftkv, so etcd is not run under clock faults.
- **LazyFS via a WAL mode** that keeps unsynced writes in process memory, so
  kill -9 loses them. Real LazyFS (FUSE) would also cover etcd.
- **No `tc netem`** in the build container (kernel module missing), so the
  0/1/10 ms network-delay runs need a machine with netem.
