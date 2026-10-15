| Cell | Attempts | Completed | Ok | Ok (reduced) | Unknown | Illegal | Checker crash (OOM) | Trials with stale reads | Stale reads | Combined detection [95% CI] | Porcupine-only detection [95% CI] | Median detection, Porcupine (s) | Median first stale read (s) | Faults planned / executed / completed | Shortest fault (s) | Bug flag on every node start |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ackbeforefsync/kill-all | 20 | 19 | 0 | 0 | 19 | 0 | 1 | 19/19 | 160 | 19/19 = 1.00 [0.83, 1.00] | 0/0 = 0.00 [0.00, 1.00] | - | 9.1 | 3/3/3 (all completed trials) | 5.00 of 5 | yes (380 starts in 19 trials) |
|  | crashed trials, offline stale-read scan of saved history: [3] | | | | | | | | | | | | | | | |
| correct-lazyfs/kill-all | 20 | 20 | 18 | 2 | 0 | 0 | 0 | 0/20 | 0 | 0/20 = 0.00 [0.00, 0.16] | 0/20 = 0.00 [0.00, 0.16] | - | - | 3/3/3 (all completed trials) | 5.00 of 5 | n/a (no bug) |
| localread/partition-leader | 20 | 16 | 1 | 1 | 13 | 1 | 4 | 14/16 | 48 | 14/16 = 0.88 [0.64, 0.97] | 1/3 = 0.33 [0.06, 0.79] | 4.2 | 4.7 | 3/3/3 (all completed trials) | 5.08 of 5 | yes (80 starts in 16 trials) |
|  | crashed trials, offline stale-read scan of saved history: [7, 14, 2, 8] | | | | | | | | | | | | | | | |
| correct/partition-leader | 20 | 20 | 16 | 4 | 0 | 0 | 0 | 0/20 | 0 | 0/20 = 0.00 [0.00, 0.16] | 0/20 = 0.00 [0.00, 0.16] | - | - | 3/3/3 (all completed trials) | 5.07 of 5 | n/a (no bug) |

## Per trial

| Cell | Trial dir | Result | Stale reads | Porcupine detection (s) |
|---|---|---|---|---|
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all/trial-001 | Unknown | 3 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all/trial-002 | Unknown | 8 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all/trial-003 | Unknown | 9 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all/trial-004 | checker crash (OOM) | 3 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-005/trial-001 | Unknown | 7 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-006/trial-001 | Unknown | 9 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-007/trial-001 | Unknown | 14 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-008/trial-001 | Unknown | 7 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-009/trial-001 | Unknown | 4 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-010/trial-001 | Unknown | 4 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-011/trial-001 | Unknown | 8 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-012/trial-001 | Unknown | 19 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-013/trial-001 | Unknown | 15 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-014/trial-001 | Unknown | 2 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-015/trial-001 | Unknown | 7 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-016/trial-001 | Unknown | 10 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-017/trial-001 | Unknown | 8 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-018/trial-001 | Unknown | 2 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-019/trial-001 | Unknown | 14 | - |
| ackbeforefsync/kill-all | E6/ackbeforefsync/kill-all-single/attempt-020/trial-001 | Unknown | 10 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-001 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-002 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-003 | Ok (reduced) | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-004 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-005 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-006 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-007 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-008 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-009 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-010 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-011 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-012 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-013 | Ok (reduced) | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-014 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-015 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-016 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-017 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-018 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-019 | Ok | 0 | - |
| correct-lazyfs/kill-all | E6/correct-lazyfs/kill-all/trial-020 | Ok | 0 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-001 | Unknown | 3 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-002 | Unknown | 2 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-003 | Illegal | 4 | 4.2 |
| localread/partition-leader | E6/localread/partition-leader/trial-004 | Unknown | 3 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-005 | Unknown | 7 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-006 | Ok (reduced) | 0 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-007 | Unknown | 3 | - |
| localread/partition-leader | E6/localread/partition-leader/trial-008 | checker crash (OOM) | 7 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-009/trial-001 | Unknown | 2 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-010/trial-001 | checker crash (OOM) | 14 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-011/trial-001 | Unknown | 3 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-012/trial-001 | checker crash (OOM) | 2 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-013/trial-001 | Unknown | 6 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-014/trial-001 | Unknown | 4 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-015/trial-001 | Unknown | 4 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-016/trial-001 | Ok | 0 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-017/trial-001 | Unknown | 1 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-018/trial-001 | Unknown | 4 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-019/trial-001 | Unknown | 2 | - |
| localread/partition-leader | E6/localread/partition-leader-single/attempt-020/trial-001 | checker crash (OOM) | 8 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-001 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-002 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-003 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-004 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-005 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-006 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-007 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-008 | Ok (reduced) | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-009 | Ok (reduced) | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-010 | Ok (reduced) | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-011 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-012 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-013 | Ok (reduced) | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-014 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-015 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-016 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-017 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-018 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-019 | Ok | 0 | - |
| correct/partition-leader | E6/correct/partition-leader/trial-020 | Ok | 0 | - |
