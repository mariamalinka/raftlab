## Outcomes per scenario

| Exp | Scenario | Trials | Ok | Ok (reduced) | Unknown | Illegal | Checker crash | Harness error | Correct (Ok + Ok red.) | Inconclusive (Unknown + crash) | Trials with stale reads | Stale reads | Illegal-or-stale [95% CI] |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| E1 | clock-skew | 20 | 19 | 1 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | kill-all-lazyfs | 20 | 16 | 4 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | kill-leader | 20 | 16 | 4 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | kill-majority | 20 | 17 | 3 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | mixed | 20 | 19 | 1 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | partition-bridge | 20 | 16 | 4 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | partition-halves | 20 | 14 | 6 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | partition-leader | 20 | 14 | 6 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E1 | pause-leader | 20 | 16 | 4 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | kill-all | 20 | 16 | 4 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | kill-leader | 20 | 19 | 1 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | kill-majority | 20 | 10 | 10 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | mixed | 20 | 8 | 11 | 1 | 0 | 0 | 0 | 19/20 | 1/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | partition-bridge | 20 | 8 | 12 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | partition-halves | 20 | 1 | 19 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |
| E2 | partition-leader | 20 | 2 | 15 | 2 | 0 | 1 | 0 | 17/20 | 3/20 | 0/19 | 0 | 0/19 [0.00, 0.17] |
| E2 | pause-leader | 20 | 4 | 16 | 0 | 0 | 0 | 0 | 20/20 | 0/20 | 0/20 | 0 | 0/20 [0.00, 0.16] |

## Client operations, faults and resources

| Exp | Scenario | Ops | ok | fail | info (indeterminate) | info share | Faults planned / executed / completed (per trial) | Fault errors | Shortest / median fault (s) | Median wall (s) | Peak RSS median / max (GB) | Crashed trials: offline stale reads |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| E1 | clock-skew | 770672 | 770397 | 89 | 186 | 0.0002 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 43 | 0.16 / 3.55 | - |
| E1 | kill-all-lazyfs | 427945 | 419333 | 2862 | 5750 | 0.0134 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 42 | 0.07 / 2.47 | - |
| E1 | kill-leader | 702141 | 700659 | 464 | 1018 | 0.0014 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 42 | 0.12 / 3.30 | - |
| E1 | kill-majority | 399679 | 390494 | 3113 | 6072 | 0.0152 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 42 | 0.03 / 3.06 | - |
| E1 | mixed | 663196 | 661596 | 516 | 1084 | 0.0016 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 42 | 0.17 / 3.45 | - |
| E1 | partition-bridge | 692165 | 691233 | 285 | 647 | 0.0009 | 3/3/3 ×20 | 0 | 5.07 / 5.09 | 43 | 0.27 / 3.80 | - |
| E1 | partition-halves | 625916 | 623935 | 646 | 1335 | 0.0021 | 3/3/3 ×20 | 0 | 5.08 / 5.10 | 44 | 0.33 / 3.18 | - |
| E1 | partition-leader | 590047 | 586845 | 911 | 2291 | 0.0039 | 3/3/3 ×20 | 0 | 5.08 / 5.10 | 43 | 0.16 / 3.75 | - |
| E1 | pause-leader | 627888 | 625150 | 886 | 1852 | 0.0029 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 43 | 0.16 / 3.04 | - |
| E2 | kill-all | 381842 | 371876 | 3243 | 6723 | 0.0176 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 45 | 0.07 / 2.41 | - |
| E2 | kill-leader | 620311 | 616670 | 954 | 2687 | 0.0043 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 46 | 0.10 / 1.49 | - |
| E2 | kill-majority | 413491 | 404603 | 2980 | 5908 | 0.0143 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 74 | 0.74 / 4.55 | - |
| E2 | mixed | 640869 | 637396 | 1025 | 2448 | 0.0038 | 3/1/1 ×1, 3/2/2 ×3, 3/3/3 ×16 | 5 | 5.00 / 5.01 | 104 | 3.81 / 10.84 | - |
| E2 | partition-bridge | 651791 | 648764 | 1049 | 1978 | 0.0030 | 3/3/3 ×20 | 0 | 5.07 / 5.08 | 103 | 3.22 / 10.88 | - |
| E2 | partition-halves | 558120 | 552990 | 1546 | 3584 | 0.0064 | 3/3/3 ×20 | 0 | 5.07 / 5.09 | 105 | 5.65 / 11.69 | - |
| E2 | partition-leader | 536068 | 531478 | 1358 | 3232 | 0.0060 | 3/3/3 ×19 | 0 | 5.08 / 5.09 | 105 | 7.43 / 13.82 | [0] |
| E2 | pause-leader | 644858 | 641699 | 774 | 2385 | 0.0037 | 3/3/3 ×20 | 0 | 5.00 / 5.00 | 106 | 4.54 / 12.67 | - |

## Per trial

| Exp | Scenario | Attempt | Result | Stale reads | Ops | Faults p/e/c | Wall (s) |
|---|---|---|---|---|---|---|---|
| E1 | clock-skew | 1 | Ok | 0 | 37346 | 3/3/3 | 41 |
| E1 | clock-skew | 2 | Ok | 0 | 40978 | 3/3/3 | 41 |
| E1 | clock-skew | 3 | Ok | 0 | 36200 | 3/3/3 | 41 |
| E1 | clock-skew | 4 | Ok | 0 | 38440 | 3/3/3 | 42 |
| E1 | clock-skew | 5 | Ok | 0 | 38943 | 3/3/3 | 48 |
| E1 | clock-skew | 6 | Ok | 0 | 39430 | 3/3/3 | 43 |
| E1 | clock-skew | 7 | Ok | 0 | 39720 | 3/3/3 | 41 |
| E1 | clock-skew | 8 | Ok | 0 | 39492 | 3/3/3 | 43 |
| E1 | clock-skew | 9 | Ok | 0 | 37028 | 3/3/3 | 41 |
| E1 | clock-skew | 10 | Ok | 0 | 38547 | 3/3/3 | 47 |
| E1 | clock-skew | 11 | Ok | 0 | 38533 | 3/3/3 | 42 |
| E1 | clock-skew | 12 | Ok | 0 | 37586 | 3/3/3 | 42 |
| E1 | clock-skew | 13 | Ok | 0 | 38979 | 3/3/3 | 41 |
| E1 | clock-skew | 14 | Ok | 0 | 40417 | 3/3/3 | 41 |
| E1 | clock-skew | 15 | Ok | 0 | 37035 | 3/3/3 | 41 |
| E1 | clock-skew | 16 | Ok | 0 | 37236 | 3/3/3 | 41 |
| E1 | clock-skew | 17 | Ok | 0 | 38972 | 3/3/3 | 41 |
| E1 | clock-skew | 18 | Ok | 0 | 36721 | 3/3/3 | 41 |
| E1 | clock-skew | 19 | Ok | 0 | 40574 | 3/3/3 | 41 |
| E1 | clock-skew | 20 | Ok (reduced) | 0 | 38495 | 3/3/3 | 102 |
| E1 | kill-all-lazyfs | 1 | Ok | 0 | 22055 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 2 | Ok | 0 | 22320 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 3 | Ok (reduced) | 0 | 21309 | 3/3/3 | 101 |
| E1 | kill-all-lazyfs | 4 | Ok | 0 | 21323 | 3/3/3 | 42 |
| E1 | kill-all-lazyfs | 5 | Ok | 0 | 20745 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 6 | Ok | 0 | 20952 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 7 | Ok | 0 | 21573 | 3/3/3 | 42 |
| E1 | kill-all-lazyfs | 8 | Ok | 0 | 21385 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 9 | Ok (reduced) | 0 | 21582 | 3/3/3 | 101 |
| E1 | kill-all-lazyfs | 10 | Ok | 0 | 21297 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 11 | Ok | 0 | 21194 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 12 | Ok | 0 | 20637 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 13 | Ok | 0 | 21180 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 14 | Ok | 0 | 21686 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 15 | Ok | 0 | 22095 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 16 | Ok | 0 | 20937 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 17 | Ok (reduced) | 0 | 21762 | 3/3/3 | 104 |
| E1 | kill-all-lazyfs | 18 | Ok | 0 | 21349 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 19 | Ok | 0 | 21119 | 3/3/3 | 41 |
| E1 | kill-all-lazyfs | 20 | Ok (reduced) | 0 | 21445 | 3/3/3 | 101 |
| E1 | kill-leader | 1 | Ok | 0 | 34773 | 3/3/3 | 41 |
| E1 | kill-leader | 2 | Ok | 0 | 35643 | 3/3/3 | 41 |
| E1 | kill-leader | 3 | Ok | 0 | 35434 | 3/3/3 | 42 |
| E1 | kill-leader | 4 | Ok | 0 | 33237 | 3/3/3 | 41 |
| E1 | kill-leader | 5 | Ok | 0 | 33908 | 3/3/3 | 41 |
| E1 | kill-leader | 6 | Ok (reduced) | 0 | 34504 | 3/3/3 | 103 |
| E1 | kill-leader | 7 | Ok | 0 | 33741 | 3/3/3 | 43 |
| E1 | kill-leader | 8 | Ok | 0 | 34855 | 3/3/3 | 42 |
| E1 | kill-leader | 9 | Ok | 0 | 34825 | 3/3/3 | 41 |
| E1 | kill-leader | 10 | Ok (reduced) | 0 | 36071 | 3/3/3 | 103 |
| E1 | kill-leader | 11 | Ok | 0 | 36067 | 3/3/3 | 41 |
| E1 | kill-leader | 12 | Ok | 0 | 34824 | 3/3/3 | 41 |
| E1 | kill-leader | 13 | Ok | 0 | 36383 | 3/3/3 | 41 |
| E1 | kill-leader | 14 | Ok | 0 | 35158 | 3/3/3 | 41 |
| E1 | kill-leader | 15 | Ok | 0 | 36208 | 3/3/3 | 41 |
| E1 | kill-leader | 16 | Ok | 0 | 36478 | 3/3/3 | 41 |
| E1 | kill-leader | 17 | Ok | 0 | 35449 | 3/3/3 | 41 |
| E1 | kill-leader | 18 | Ok (reduced) | 0 | 35689 | 3/3/3 | 102 |
| E1 | kill-leader | 19 | Ok (reduced) | 0 | 34721 | 3/3/3 | 104 |
| E1 | kill-leader | 20 | Ok | 0 | 34173 | 3/3/3 | 41 |
| E1 | kill-majority | 1 | Ok | 0 | 21730 | 3/3/3 | 41 |
| E1 | kill-majority | 2 | Ok | 0 | 19896 | 3/3/3 | 41 |
| E1 | kill-majority | 3 | Ok | 0 | 20557 | 3/3/3 | 41 |
| E1 | kill-majority | 4 | Ok | 0 | 18110 | 3/3/3 | 41 |
| E1 | kill-majority | 5 | Ok | 0 | 18978 | 3/3/3 | 41 |
| E1 | kill-majority | 6 | Ok | 0 | 19905 | 3/3/3 | 41 |
| E1 | kill-majority | 7 | Ok | 0 | 20613 | 3/3/3 | 41 |
| E1 | kill-majority | 8 | Ok | 0 | 19687 | 3/3/3 | 42 |
| E1 | kill-majority | 9 | Ok (reduced) | 0 | 21798 | 3/3/3 | 101 |
| E1 | kill-majority | 10 | Ok | 0 | 20713 | 3/3/3 | 41 |
| E1 | kill-majority | 11 | Ok | 0 | 19702 | 3/3/3 | 41 |
| E1 | kill-majority | 12 | Ok | 0 | 18569 | 3/3/3 | 41 |
| E1 | kill-majority | 13 | Ok (reduced) | 0 | 20142 | 3/3/3 | 101 |
| E1 | kill-majority | 14 | Ok | 0 | 19330 | 3/3/3 | 41 |
| E1 | kill-majority | 15 | Ok | 0 | 18345 | 3/3/3 | 41 |
| E1 | kill-majority | 16 | Ok | 0 | 20388 | 3/3/3 | 41 |
| E1 | kill-majority | 17 | Ok | 0 | 19711 | 3/3/3 | 42 |
| E1 | kill-majority | 18 | Ok (reduced) | 0 | 20022 | 3/3/3 | 102 |
| E1 | kill-majority | 19 | Ok | 0 | 20315 | 3/3/3 | 42 |
| E1 | kill-majority | 20 | Ok | 0 | 21168 | 3/3/3 | 41 |
| E1 | mixed | 1 | Ok | 0 | 34909 | 3/3/3 | 41 |
| E1 | mixed | 2 | Ok | 0 | 38523 | 3/3/3 | 41 |
| E1 | mixed | 3 | Ok | 0 | 33631 | 3/3/3 | 41 |
| E1 | mixed | 4 | Ok | 0 | 30734 | 3/3/3 | 42 |
| E1 | mixed | 5 | Ok | 0 | 30314 | 3/3/3 | 41 |
| E1 | mixed | 6 | Ok | 0 | 33799 | 3/3/3 | 41 |
| E1 | mixed | 7 | Ok | 0 | 35141 | 3/3/3 | 41 |
| E1 | mixed | 8 | Ok | 0 | 33747 | 3/3/3 | 41 |
| E1 | mixed | 9 | Ok | 0 | 34470 | 3/3/3 | 42 |
| E1 | mixed | 10 | Ok | 0 | 31538 | 3/3/3 | 41 |
| E1 | mixed | 11 | Ok | 0 | 30564 | 3/3/3 | 54 |
| E1 | mixed | 12 | Ok | 0 | 29151 | 3/3/3 | 41 |
| E1 | mixed | 13 | Ok | 0 | 33385 | 3/3/3 | 41 |
| E1 | mixed | 14 | Ok | 0 | 33635 | 3/3/3 | 44 |
| E1 | mixed | 15 | Ok | 0 | 35239 | 3/3/3 | 41 |
| E1 | mixed | 16 | Ok | 0 | 32878 | 3/3/3 | 41 |
| E1 | mixed | 17 | Ok | 0 | 33088 | 3/3/3 | 44 |
| E1 | mixed | 18 | Ok | 0 | 35444 | 3/3/3 | 42 |
| E1 | mixed | 19 | Ok | 0 | 31993 | 3/3/3 | 41 |
| E1 | mixed | 20 | Ok (reduced) | 0 | 31013 | 3/3/3 | 105 |
| E1 | partition-bridge | 1 | Ok | 0 | 36038 | 3/3/3 | 41 |
| E1 | partition-bridge | 2 | Ok | 0 | 33702 | 3/3/3 | 55 |
| E1 | partition-bridge | 3 | Ok | 0 | 34461 | 3/3/3 | 53 |
| E1 | partition-bridge | 4 | Ok | 0 | 39016 | 3/3/3 | 41 |
| E1 | partition-bridge | 5 | Ok | 0 | 35847 | 3/3/3 | 41 |
| E1 | partition-bridge | 6 | Ok | 0 | 35141 | 3/3/3 | 41 |
| E1 | partition-bridge | 7 | Ok | 0 | 34866 | 3/3/3 | 42 |
| E1 | partition-bridge | 8 | Ok | 0 | 30461 | 3/3/3 | 42 |
| E1 | partition-bridge | 9 | Ok (reduced) | 0 | 32159 | 3/3/3 | 102 |
| E1 | partition-bridge | 10 | Ok | 0 | 36734 | 3/3/3 | 41 |
| E1 | partition-bridge | 11 | Ok | 0 | 32261 | 3/3/3 | 42 |
| E1 | partition-bridge | 12 | Ok | 0 | 32652 | 3/3/3 | 41 |
| E1 | partition-bridge | 13 | Ok (reduced) | 0 | 33733 | 3/3/3 | 103 |
| E1 | partition-bridge | 14 | Ok | 0 | 32227 | 3/3/3 | 83 |
| E1 | partition-bridge | 15 | Ok | 0 | 31613 | 3/3/3 | 42 |
| E1 | partition-bridge | 16 | Ok | 0 | 38167 | 3/3/3 | 41 |
| E1 | partition-bridge | 17 | Ok (reduced) | 0 | 33139 | 3/3/3 | 104 |
| E1 | partition-bridge | 18 | Ok | 0 | 36251 | 3/3/3 | 42 |
| E1 | partition-bridge | 19 | Ok | 0 | 38041 | 3/3/3 | 41 |
| E1 | partition-bridge | 20 | Ok (reduced) | 0 | 35656 | 3/3/3 | 104 |
| E1 | partition-halves | 1 | Ok (reduced) | 0 | 31122 | 3/3/3 | 103 |
| E1 | partition-halves | 2 | Ok | 0 | 30549 | 3/3/3 | 41 |
| E1 | partition-halves | 3 | Ok | 0 | 31232 | 3/3/3 | 42 |
| E1 | partition-halves | 4 | Ok | 0 | 30160 | 3/3/3 | 42 |
| E1 | partition-halves | 5 | Ok | 0 | 33229 | 3/3/3 | 41 |
| E1 | partition-halves | 6 | Ok | 0 | 33561 | 3/3/3 | 41 |
| E1 | partition-halves | 7 | Ok (reduced) | 0 | 31344 | 3/3/3 | 102 |
| E1 | partition-halves | 8 | Ok (reduced) | 0 | 32970 | 3/3/3 | 102 |
| E1 | partition-halves | 9 | Ok | 0 | 30480 | 3/3/3 | 44 |
| E1 | partition-halves | 10 | Ok (reduced) | 0 | 32445 | 3/3/3 | 102 |
| E1 | partition-halves | 11 | Ok | 0 | 32554 | 3/3/3 | 41 |
| E1 | partition-halves | 12 | Ok (reduced) | 0 | 31504 | 3/3/3 | 102 |
| E1 | partition-halves | 13 | Ok | 0 | 31725 | 3/3/3 | 41 |
| E1 | partition-halves | 14 | Ok | 0 | 31603 | 3/3/3 | 41 |
| E1 | partition-halves | 15 | Ok | 0 | 28991 | 3/3/3 | 41 |
| E1 | partition-halves | 16 | Ok | 0 | 30516 | 3/3/3 | 44 |
| E1 | partition-halves | 17 | Ok (reduced) | 0 | 31312 | 3/3/3 | 105 |
| E1 | partition-halves | 18 | Ok | 0 | 30068 | 3/3/3 | 61 |
| E1 | partition-halves | 19 | Ok | 0 | 29222 | 3/3/3 | 43 |
| E1 | partition-halves | 20 | Ok | 0 | 31329 | 3/3/3 | 72 |
| E1 | partition-leader | 1 | Ok | 0 | 31267 | 3/3/3 | 41 |
| E1 | partition-leader | 2 | Ok (reduced) | 0 | 30706 | 3/3/3 | 104 |
| E1 | partition-leader | 3 | Ok | 0 | 29709 | 3/3/3 | 41 |
| E1 | partition-leader | 4 | Ok | 0 | 29118 | 3/3/3 | 42 |
| E1 | partition-leader | 5 | Ok | 0 | 28532 | 3/3/3 | 42 |
| E1 | partition-leader | 6 | Ok (reduced) | 0 | 28836 | 3/3/3 | 104 |
| E1 | partition-leader | 7 | Ok | 0 | 28188 | 3/3/3 | 41 |
| E1 | partition-leader | 8 | Ok | 0 | 29402 | 3/3/3 | 41 |
| E1 | partition-leader | 9 | Ok | 0 | 29053 | 3/3/3 | 43 |
| E1 | partition-leader | 10 | Ok | 0 | 30095 | 3/3/3 | 41 |
| E1 | partition-leader | 11 | Ok | 0 | 29964 | 3/3/3 | 41 |
| E1 | partition-leader | 12 | Ok (reduced) | 0 | 30118 | 3/3/3 | 102 |
| E1 | partition-leader | 13 | Ok (reduced) | 0 | 28023 | 3/3/3 | 103 |
| E1 | partition-leader | 14 | Ok | 0 | 29569 | 3/3/3 | 41 |
| E1 | partition-leader | 15 | Ok | 0 | 27965 | 3/3/3 | 41 |
| E1 | partition-leader | 16 | Ok (reduced) | 0 | 30722 | 3/3/3 | 102 |
| E1 | partition-leader | 17 | Ok | 0 | 29701 | 3/3/3 | 41 |
| E1 | partition-leader | 18 | Ok | 0 | 29771 | 3/3/3 | 41 |
| E1 | partition-leader | 19 | Ok (reduced) | 0 | 29593 | 3/3/3 | 101 |
| E1 | partition-leader | 20 | Ok | 0 | 29715 | 3/3/3 | 41 |
| E1 | pause-leader | 1 | Ok | 0 | 31205 | 3/3/3 | 41 |
| E1 | pause-leader | 2 | Ok | 0 | 31283 | 3/3/3 | 42 |
| E1 | pause-leader | 3 | Ok (reduced) | 0 | 30833 | 3/3/3 | 102 |
| E1 | pause-leader | 4 | Ok | 0 | 33653 | 3/3/3 | 41 |
| E1 | pause-leader | 5 | Ok | 0 | 30882 | 3/3/3 | 41 |
| E1 | pause-leader | 6 | Ok | 0 | 31421 | 3/3/3 | 41 |
| E1 | pause-leader | 7 | Ok | 0 | 30985 | 3/3/3 | 41 |
| E1 | pause-leader | 8 | Ok | 0 | 32190 | 3/3/3 | 42 |
| E1 | pause-leader | 9 | Ok | 0 | 31024 | 3/3/3 | 47 |
| E1 | pause-leader | 10 | Ok | 0 | 30387 | 3/3/3 | 42 |
| E1 | pause-leader | 11 | Ok | 0 | 32194 | 3/3/3 | 52 |
| E1 | pause-leader | 12 | Ok | 0 | 30426 | 3/3/3 | 42 |
| E1 | pause-leader | 13 | Ok | 0 | 31152 | 3/3/3 | 42 |
| E1 | pause-leader | 14 | Ok (reduced) | 0 | 32712 | 3/3/3 | 107 |
| E1 | pause-leader | 15 | Ok | 0 | 33447 | 3/3/3 | 41 |
| E1 | pause-leader | 16 | Ok (reduced) | 0 | 28283 | 3/3/3 | 102 |
| E1 | pause-leader | 17 | Ok | 0 | 30514 | 3/3/3 | 41 |
| E1 | pause-leader | 18 | Ok | 0 | 32826 | 3/3/3 | 41 |
| E1 | pause-leader | 19 | Ok (reduced) | 0 | 32074 | 3/3/3 | 102 |
| E1 | pause-leader | 20 | Ok | 0 | 30397 | 3/3/3 | 41 |
| E2 | kill-all | 1 | Ok | 0 | 19215 | 3/3/3 | 44 |
| E2 | kill-all | 2 | Ok | 0 | 18330 | 3/3/3 | 44 |
| E2 | kill-all | 3 | Ok | 0 | 19375 | 3/3/3 | 49 |
| E2 | kill-all | 4 | Ok | 0 | 18620 | 3/3/3 | 44 |
| E2 | kill-all | 5 | Ok (reduced) | 0 | 18887 | 3/3/3 | 100 |
| E2 | kill-all | 6 | Ok | 0 | 18155 | 3/3/3 | 46 |
| E2 | kill-all | 7 | Ok | 0 | 19781 | 3/3/3 | 44 |
| E2 | kill-all | 8 | Ok | 0 | 19528 | 3/3/3 | 44 |
| E2 | kill-all | 9 | Ok (reduced) | 0 | 18528 | 3/3/3 | 104 |
| E2 | kill-all | 10 | Ok | 0 | 19438 | 3/3/3 | 45 |
| E2 | kill-all | 11 | Ok | 0 | 19755 | 3/3/3 | 44 |
| E2 | kill-all | 12 | Ok | 0 | 19167 | 3/3/3 | 44 |
| E2 | kill-all | 13 | Ok | 0 | 18775 | 3/3/3 | 44 |
| E2 | kill-all | 14 | Ok (reduced) | 0 | 18235 | 3/3/3 | 103 |
| E2 | kill-all | 15 | Ok | 0 | 19312 | 3/3/3 | 44 |
| E2 | kill-all | 16 | Ok | 0 | 18211 | 3/3/3 | 44 |
| E2 | kill-all | 17 | Ok (reduced) | 0 | 20275 | 3/3/3 | 103 |
| E2 | kill-all | 18 | Ok | 0 | 18728 | 3/3/3 | 41 |
| E2 | kill-all | 19 | Ok | 0 | 19784 | 3/3/3 | 44 |
| E2 | kill-all | 20 | Ok | 0 | 19743 | 3/3/3 | 44 |
| E2 | kill-leader | 1 | Ok | 0 | 31814 | 3/3/3 | 44 |
| E2 | kill-leader | 2 | Ok | 0 | 32991 | 3/3/3 | 49 |
| E2 | kill-leader | 3 | Ok | 0 | 31098 | 3/3/3 | 44 |
| E2 | kill-leader | 4 | Ok | 0 | 30184 | 3/3/3 | 64 |
| E2 | kill-leader | 5 | Ok | 0 | 31332 | 3/3/3 | 44 |
| E2 | kill-leader | 6 | Ok | 0 | 30557 | 3/3/3 | 40 |
| E2 | kill-leader | 7 | Ok | 0 | 30847 | 3/3/3 | 45 |
| E2 | kill-leader | 8 | Ok | 0 | 30934 | 3/3/3 | 44 |
| E2 | kill-leader | 9 | Ok | 0 | 32508 | 3/3/3 | 44 |
| E2 | kill-leader | 10 | Ok | 0 | 30605 | 3/3/3 | 40 |
| E2 | kill-leader | 11 | Ok | 0 | 30457 | 3/3/3 | 45 |
| E2 | kill-leader | 12 | Ok | 0 | 30771 | 3/3/3 | 44 |
| E2 | kill-leader | 13 | Ok | 0 | 31445 | 3/3/3 | 44 |
| E2 | kill-leader | 14 | Ok | 0 | 30280 | 3/3/3 | 44 |
| E2 | kill-leader | 15 | Ok | 0 | 31050 | 3/3/3 | 55 |
| E2 | kill-leader | 16 | Ok | 0 | 30536 | 3/3/3 | 44 |
| E2 | kill-leader | 17 | Ok (reduced) | 0 | 30689 | 3/3/3 | 105 |
| E2 | kill-leader | 18 | Ok | 0 | 31298 | 3/3/3 | 40 |
| E2 | kill-leader | 19 | Ok | 0 | 30300 | 3/3/3 | 45 |
| E2 | kill-leader | 20 | Ok | 0 | 30615 | 3/3/3 | 44 |
| E2 | kill-majority | 1 | Ok | 0 | 20427 | 3/3/3 | 44 |
| E2 | kill-majority | 2 | Ok | 0 | 20939 | 3/3/3 | 40 |
| E2 | kill-majority | 3 | Ok (reduced) | 0 | 20876 | 3/3/3 | 104 |
| E2 | kill-majority | 4 | Ok (reduced) | 0 | 20750 | 3/3/3 | 104 |
| E2 | kill-majority | 5 | Ok (reduced) | 0 | 20206 | 3/3/3 | 100 |
| E2 | kill-majority | 6 | Ok (reduced) | 0 | 20119 | 3/3/3 | 103 |
| E2 | kill-majority | 7 | Ok | 0 | 20895 | 3/3/3 | 44 |
| E2 | kill-majority | 8 | Ok | 0 | 20656 | 3/3/3 | 40 |
| E2 | kill-majority | 9 | Ok (reduced) | 0 | 19646 | 3/3/3 | 104 |
| E2 | kill-majority | 10 | Ok (reduced) | 0 | 20066 | 3/3/3 | 104 |
| E2 | kill-majority | 11 | Ok | 0 | 20796 | 3/3/3 | 41 |
| E2 | kill-majority | 12 | Ok | 0 | 20961 | 3/3/3 | 46 |
| E2 | kill-majority | 13 | Ok | 0 | 19691 | 3/3/3 | 43 |
| E2 | kill-majority | 14 | Ok (reduced) | 0 | 20580 | 3/3/3 | 104 |
| E2 | kill-majority | 15 | Ok | 0 | 20909 | 3/3/3 | 44 |
| E2 | kill-majority | 16 | Ok (reduced) | 0 | 21453 | 3/3/3 | 105 |
| E2 | kill-majority | 17 | Ok (reduced) | 0 | 21270 | 3/3/3 | 103 |
| E2 | kill-majority | 18 | Ok (reduced) | 0 | 21366 | 3/3/3 | 102 |
| E2 | kill-majority | 19 | Ok | 0 | 21861 | 3/3/3 | 44 |
| E2 | kill-majority | 20 | Ok | 0 | 20024 | 3/3/3 | 45 |
| E2 | mixed | 1 | Ok (reduced) | 0 | 35884 | 3/2/2 | 104 |
| E2 | mixed | 2 | Ok | 0 | 32864 | 3/2/2 | 40 |
| E2 | mixed | 3 | Ok (reduced) | 0 | 30094 | 3/3/3 | 104 |
| E2 | mixed | 4 | Ok (reduced) | 0 | 28130 | 3/3/3 | 102 |
| E2 | mixed | 5 | Ok | 0 | 33456 | 3/2/2 | 40 |
| E2 | mixed | 6 | Ok (reduced) | 0 | 28886 | 3/3/3 | 105 |
| E2 | mixed | 7 | Ok (reduced) | 0 | 33140 | 3/3/3 | 100 |
| E2 | mixed | 8 | Ok | 0 | 31705 | 3/3/3 | 45 |
| E2 | mixed | 9 | Ok (reduced) | 0 | 32060 | 3/3/3 | 105 |
| E2 | mixed | 10 | Ok (reduced) | 0 | 36010 | 3/3/3 | 104 |
| E2 | mixed | 11 | Unknown | 0 | 28116 | 3/3/3 | 105 |
| E2 | mixed | 12 | Ok (reduced) | 0 | 32183 | 3/3/3 | 100 |
| E2 | mixed | 13 | Ok | 0 | 32695 | 3/3/3 | 102 |
| E2 | mixed | 14 | Ok | 0 | 32081 | 3/3/3 | 40 |
| E2 | mixed | 15 | Ok (reduced) | 0 | 32866 | 3/3/3 | 105 |
| E2 | mixed | 16 | Ok | 0 | 30883 | 3/3/3 | 44 |
| E2 | mixed | 17 | Ok (reduced) | 0 | 31174 | 3/3/3 | 104 |
| E2 | mixed | 18 | Ok | 0 | 36933 | 3/1/1 | 44 |
| E2 | mixed | 19 | Ok | 0 | 33079 | 3/3/3 | 44 |
| E2 | mixed | 20 | Ok (reduced) | 0 | 28630 | 3/3/3 | 104 |
| E2 | partition-bridge | 1 | Ok (reduced) | 0 | 30889 | 3/3/3 | 104 |
| E2 | partition-bridge | 2 | Ok | 0 | 32857 | 3/3/3 | 42 |
| E2 | partition-bridge | 3 | Ok | 0 | 31537 | 3/3/3 | 44 |
| E2 | partition-bridge | 4 | Ok (reduced) | 0 | 33064 | 3/3/3 | 102 |
| E2 | partition-bridge | 5 | Ok (reduced) | 0 | 31829 | 3/3/3 | 106 |
| E2 | partition-bridge | 6 | Ok (reduced) | 0 | 30954 | 3/3/3 | 104 |
| E2 | partition-bridge | 7 | Ok (reduced) | 0 | 28740 | 3/3/3 | 102 |
| E2 | partition-bridge | 8 | Ok | 0 | 31522 | 3/3/3 | 43 |
| E2 | partition-bridge | 9 | Ok (reduced) | 0 | 34398 | 3/3/3 | 105 |
| E2 | partition-bridge | 10 | Ok (reduced) | 0 | 30134 | 3/3/3 | 100 |
| E2 | partition-bridge | 11 | Ok (reduced) | 0 | 33738 | 3/3/3 | 104 |
| E2 | partition-bridge | 12 | Ok | 0 | 37226 | 3/3/3 | 44 |
| E2 | partition-bridge | 13 | Ok (reduced) | 0 | 28855 | 3/3/3 | 105 |
| E2 | partition-bridge | 14 | Ok | 0 | 36796 | 3/3/3 | 41 |
| E2 | partition-bridge | 15 | Ok (reduced) | 0 | 29819 | 3/3/3 | 106 |
| E2 | partition-bridge | 16 | Ok | 0 | 32767 | 3/3/3 | 43 |
| E2 | partition-bridge | 17 | Ok | 0 | 36632 | 3/3/3 | 44 |
| E2 | partition-bridge | 18 | Ok (reduced) | 0 | 33133 | 3/3/3 | 105 |
| E2 | partition-bridge | 19 | Ok | 0 | 34894 | 3/3/3 | 44 |
| E2 | partition-bridge | 20 | Ok (reduced) | 0 | 32007 | 3/3/3 | 104 |
| E2 | partition-halves | 1 | Ok (reduced) | 0 | 27339 | 3/3/3 | 101 |
| E2 | partition-halves | 2 | Ok (reduced) | 0 | 26115 | 3/3/3 | 104 |
| E2 | partition-halves | 3 | Ok (reduced) | 0 | 26804 | 3/3/3 | 105 |
| E2 | partition-halves | 4 | Ok (reduced) | 0 | 24097 | 3/3/3 | 104 |
| E2 | partition-halves | 5 | Ok (reduced) | 0 | 28063 | 3/3/3 | 105 |
| E2 | partition-halves | 6 | Ok (reduced) | 0 | 24949 | 3/3/3 | 104 |
| E2 | partition-halves | 7 | Ok (reduced) | 0 | 28568 | 3/3/3 | 103 |
| E2 | partition-halves | 8 | Ok (reduced) | 0 | 27777 | 3/3/3 | 104 |
| E2 | partition-halves | 9 | Ok (reduced) | 0 | 27155 | 3/3/3 | 104 |
| E2 | partition-halves | 10 | Ok (reduced) | 0 | 28554 | 3/3/3 | 101 |
| E2 | partition-halves | 11 | Ok | 0 | 27835 | 3/3/3 | 42 |
| E2 | partition-halves | 12 | Ok (reduced) | 0 | 29257 | 3/3/3 | 107 |
| E2 | partition-halves | 13 | Ok (reduced) | 0 | 27947 | 3/3/3 | 101 |
| E2 | partition-halves | 14 | Ok (reduced) | 0 | 24767 | 3/3/3 | 133 |
| E2 | partition-halves | 15 | Ok (reduced) | 0 | 30005 | 3/3/3 | 104 |
| E2 | partition-halves | 16 | Ok (reduced) | 0 | 29989 | 3/3/3 | 100 |
| E2 | partition-halves | 17 | Ok (reduced) | 0 | 30455 | 3/3/3 | 102 |
| E2 | partition-halves | 18 | Ok (reduced) | 0 | 31103 | 3/3/3 | 104 |
| E2 | partition-halves | 19 | Ok (reduced) | 0 | 27822 | 3/3/3 | 105 |
| E2 | partition-halves | 20 | Ok (reduced) | 0 | 29519 | 3/3/3 | 105 |
| E2 | partition-leader | 1 | Ok (reduced) | 0 | 27915 | 3/3/3 | 102 |
| E2 | partition-leader | 2 | Ok (reduced) | 0 | 27468 | 3/3/3 | 103 |
| E2 | partition-leader | 3 | Ok (reduced) | 0 | 28564 | 3/3/3 | 106 |
| E2 | partition-leader | 4 | Ok (reduced) | 0 | 29631 | 3/3/3 | 100 |
| E2 | partition-leader | 5 | checker crash | 0 | 28642 | - | - |
| E2 | partition-leader | 6 | Ok (reduced) | 0 | 27326 | 3/3/3 | 104 |
| E2 | partition-leader | 7 | Ok (reduced) | 0 | 26953 | 3/3/3 | 101 |
| E2 | partition-leader | 8 | Unknown | 0 | 28373 | 3/3/3 | 104 |
| E2 | partition-leader | 9 | Ok (reduced) | 0 | 27612 | 3/3/3 | 105 |
| E2 | partition-leader | 10 | Ok (reduced) | 0 | 27357 | 3/3/3 | 113 |
| E2 | partition-leader | 11 | Ok (reduced) | 0 | 28384 | 3/3/3 | 104 |
| E2 | partition-leader | 12 | Ok (reduced) | 0 | 29035 | 3/3/3 | 105 |
| E2 | partition-leader | 13 | Ok (reduced) | 0 | 28452 | 3/3/3 | 104 |
| E2 | partition-leader | 14 | Ok | 0 | 29307 | 3/3/3 | 41 |
| E2 | partition-leader | 15 | Ok | 0 | 28227 | 3/3/3 | 45 |
| E2 | partition-leader | 16 | Ok (reduced) | 0 | 27770 | 3/3/3 | 100 |
| E2 | partition-leader | 17 | Ok (reduced) | 0 | 27849 | 3/3/3 | 103 |
| E2 | partition-leader | 18 | Ok (reduced) | 0 | 28312 | 3/3/3 | 105 |
| E2 | partition-leader | 19 | Unknown | 0 | 28202 | 3/3/3 | 102 |
| E2 | partition-leader | 20 | Ok (reduced) | 0 | 29331 | 3/3/3 | 106 |
| E2 | pause-leader | 1 | Ok | 0 | 33535 | 3/3/3 | 43 |
| E2 | pause-leader | 2 | Ok (reduced) | 0 | 32793 | 3/3/3 | 102 |
| E2 | pause-leader | 3 | Ok (reduced) | 0 | 32004 | 3/3/3 | 104 |
| E2 | pause-leader | 4 | Ok (reduced) | 0 | 31462 | 3/3/3 | 102 |
| E2 | pause-leader | 5 | Ok | 0 | 32381 | 3/3/3 | 42 |
| E2 | pause-leader | 6 | Ok (reduced) | 0 | 32676 | 3/3/3 | 106 |
| E2 | pause-leader | 7 | Ok (reduced) | 0 | 32978 | 3/3/3 | 102 |
| E2 | pause-leader | 8 | Ok (reduced) | 0 | 31541 | 3/3/3 | 110 |
| E2 | pause-leader | 9 | Ok | 0 | 31735 | 3/3/3 | 46 |
| E2 | pause-leader | 10 | Ok (reduced) | 0 | 31489 | 3/3/3 | 107 |
| E2 | pause-leader | 11 | Ok | 0 | 32461 | 3/3/3 | 44 |
| E2 | pause-leader | 12 | Ok (reduced) | 0 | 31434 | 3/3/3 | 101 |
| E2 | pause-leader | 13 | Ok (reduced) | 0 | 33153 | 3/3/3 | 105 |
| E2 | pause-leader | 14 | Ok (reduced) | 0 | 32207 | 3/3/3 | 102 |
| E2 | pause-leader | 15 | Ok (reduced) | 0 | 31132 | 3/3/3 | 105 |
| E2 | pause-leader | 16 | Ok (reduced) | 0 | 31922 | 3/3/3 | 105 |
| E2 | pause-leader | 17 | Ok (reduced) | 0 | 31679 | 3/3/3 | 106 |
| E2 | pause-leader | 18 | Ok (reduced) | 0 | 31492 | 3/3/3 | 105 |
| E2 | pause-leader | 19 | Ok (reduced) | 0 | 33661 | 3/3/3 | 105 |
| E2 | pause-leader | 20 | Ok (reduced) | 0 | 33123 | 3/3/3 | 106 |
