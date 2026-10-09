# Threats to validity

Running list; the report's section is written from it.

## Internal

- **Single machine.** All nodes share one kernel, disk and CPU set (network
  namespaces, not separate hosts). Disk contention between five WALs and CPU
  contention with the load generator affect both systems, but not
  necessarily equally.
- **Our harness, not Jepsen.** Fault injection and the checker are our own
  code (Porcupine is third-party). E2 (etcd must pass) is the control for
  harness false positives; E6 (injected bugs must be found) is the control
  for false negatives.
- **Clock faults are simulated in-process.** The skewable clock changes what
  raftkv's timers see, not the system clock. It cannot reproduce effects of
  real NTP steps on the Go runtime or the kernel.
- **LazyFS stand-in.** Lost unsynced writes are modelled by the WAL holding
  them in memory until fsync. A real power loss can also tear writes or
  reorder them; our WAL's checksums handle tearing, reordering is not
  modelled.
- **Checker timeouts.** Porcupine can time out on long or highly concurrent
  histories; such trials are reported as "unknown", never as passes.
- **Bounded run lengths.** Trials are 20–30 s with 3–5 s faults. Bugs that
  need longer schedules (for example several overlapping elections) may be
  under-detected.

## Construct

- **Codec difference.** raftkv uses a hand-written binary codec over gRPC;
  etcd uses protobuf. Part of any latency gap is encoding.
- **Feature difference.** etcd maintains MVCC history, watches and leases;
  raftkv keeps a plain map. etcd does more work per write.
- **Workload.** Register workload on few keys at a time with a fixed op mix
  (1/3 read, write, CAS). Detection rates in E6 depend on it (bug journal
  H-1).

## External

- Results are for one etcd release (v3.7.2, recorded with every result) and
  default timeouts (1 s election, 100 ms heartbeat).
- No real network delay (`tc netem` was unavailable in the build container);
  everything ran over a local bridge.

Claims are scoped to what was tested. Nothing here is "proven correct".
