| Cell | Trial | Bugs on every start | Faults planned / executed / completed | Shortest fault (s) of configured | Two leaders in a term | Suffix truncations | Checker | Stale reads |
|---|---|---|---|---|---|---|---|---|
| ackbeforefsync/kill-all | 1 | yes (20 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 2 | Unknown | 5 |
| ackbeforefsync/kill-all | 2 | yes (20 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 1 | Unknown | 9 |
| ackbeforefsync/kill-all | 3 | yes (20 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Unknown | 7 |
| ackbeforefsync/kill-majority | 1 | yes (14 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 1 | Ok | 0 |
| ackbeforefsync/kill-majority | 2 | yes (14 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 1 | Ok | 0 |
| ackbeforefsync/kill-majority | 3 | yes (14 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |
| correct-lease/clock-partition-mild | 1 | yes (5 starts) | 3 / 3 / 3 | 5.09 of 5 | - | 2 | Ok (reduced) | 0 |
| correct-lease/clock-partition-mild | 2 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 5 | Ok (reduced) | 0 |
| correct-lease/clock-partition-mild | 3 | yes (5 starts) | 3 / 3 / 3 | 5.10 of 5 | - | 3 | Ok | 0 |
| correct/kill-all-lazyfs | 1 | yes (20 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 2 | Ok (reduced) | 0 |
| correct/kill-all-lazyfs | 2 | yes (20 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |
| correct/kill-all-lazyfs | 3 | yes (20 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |
| correct/partition-leader | 1 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 2 | Ok | 0 |
| correct/partition-leader | 2 | yes (5 starts) | 3 / 3 / 3 | 5.07 of 5 | - | 2 | Ok (reduced) | 0 |
| correct/partition-leader | 3 | yes (5 starts) | 3 / 3 / 3 | 5.09 of 5 | - | 2 | Ok (reduced) | 0 |
| figure8/mixed | 1 | yes (5 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |
| figure8/mixed | 2 | yes (5 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 2 | Ok | 0 |
| figure8/mixed | 3 | yes (6 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 1 | Ok | 0 |
| leasenodrift/clock-partition-mild | 1 | yes (5 starts) | 3 / 3 / 3 | 5.10 of 5 | - | 2 | Ok (reduced) | 0 |
| leasenodrift/clock-partition-mild | 2 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 2 | Illegal | 1 |
| leasenodrift/clock-partition-mild | 3 | yes (5 starts) | 3 / 3 / 3 | 5.10 of 5 | - | 2 | Ok | 0 |
| localread/partition-leader | 1 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 2 | Ok | 0 |
| localread/partition-leader | 2 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 4 | Illegal | 8 |
| localread/partition-leader | 3 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 3 | Unknown | 1 |
| nodedup/pause-leader | 1 | yes (5 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 2 | Ok (reduced) | 0 |
| nodedup/pause-leader | 2 | yes (5 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |
| nodedup/pause-leader | 3 | yes (5 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 1 | Ok | 0 |
| notruncate/partition-halves | 1 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 0 | Ok | 0 |
| notruncate/partition-halves | 2 | yes (5 starts) | 3 / 3 / 3 | 5.09 of 5 | - | 2 | Ok | 0 |
| notruncate/partition-halves | 3 | yes (5 starts) | 3 / 3 / 3 | 5.08 of 5 | - | 2 | Ok | 0 |
| votenopersist/kill-majority | 1 | yes (14 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 1 | Ok | 0 |
| votenopersist/kill-majority | 2 | yes (14 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |
| votenopersist/kill-majority | 3 | yes (14 starts) | 3 / 3 / 3 | 5.00 of 5 | - | 0 | Ok | 0 |

Trials with a readiness problem: 0
