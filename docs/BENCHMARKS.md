# Batch dispatch experiment

Raw output: [local.jsonl](benchmarks/local.jsonl).

Recorded on 2026-09-04 America/Chicago (2026-09-05 UTC), Apple M2, macOS 26.5.2,
Go 1.27.1. One uncontrolled desktop run per seed, no CPU pinning or statistical
repetitions. Reproduce with `go run ./cmd/experiment`.

Each seed builds a synthetic 30 × 30 bidirectional road grid with 1,000 ms per
edge, 150 drivers, and 150 requests with pickup deadlines from 3,000 to 15,000 ms.
Both strategies use the same road routes, availability constraints, feasibility
filter, and snapshot. Greedy processes request IDs in order, selecting the nearest
eligible free driver. Batch matching maximizes served requests first, then
minimizes aggregate wait among assignments of that cardinality.

| Seed | Greedy served | Batch served | Greedy wait ms | Batch wait ms | Greedy solve ms | Batch solve ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 7 | 137 | 150 | 351,000 | 438,000 | 21.901 | 33.270 |
| 19 | 140 | 149 | 388,000 | 383,000 | 21.029 | 32.389 |
| 41 | 134 | 150 | 335,000 | 491,000 | 21.088 | 32.191 |

Totals: **449/450 served by batch matching, 411/450 by greedy**. Batch optimization
adds solver time and can increase aggregate wait by serving more requests. Wait
sums are over different served sets; they are not an apples-to-apples latency
comparison. The objective does not guarantee smaller maximum individual wait.
Solve timing includes routing, feasible-pair construction, and assignment, while
graph construction is outside the timed region.

The respective seeds produce 3,683, 3,505, and 3,074 feasible pairs and perform 289,
283, and 288 routing queries. Routing vectors are reused only for the current
source; the implementation does not retain a quadratic all-sources distance table.

This is a grid experiment, not a city map or historical trip replay. It excludes
networking, live driver acceptance, traffic prediction, pooled rides, demand
forecasting, pricing, and transactional assignment. Serving more riders on this
workload does not establish improvement on a production marketplace. The exact
small-instance oracle tests establish the implemented objective independently of
these benchmark outcomes.
