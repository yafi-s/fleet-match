# Routing, feasibility, and assignment

For each driver, routing computes travel times from its current node. Requests
sharing a pickup node share one routing query for their trip durations. A pair is
feasible only when both the pickup and dropoff are reachable, and
`max(now, driver.available_at) + pickup_travel <= request.latest_pickup`.
Edge weights are static throughout a snapshot; route cost is not reevaluated at
future departure times. Wait cost is pickup time minus the original request time.

The residual network has source→driver, driver→rider, and rider→sink edges, all
with unit capacities. Only driver→rider edges have initial nonzero costs. Every
augmentation adds one served request. The algorithm continues until no augmenting
path exists, obtaining maximum cardinality. Shortest residual augmentations
minimize cost at that cardinality. Reverse arcs permit earlier matches to be
reassigned; the greedy dead-end example depends on this ability.

All initial costs are nonnegative. Dijkstra then uses node potentials to maintain
nonnegative reduced costs as reverse arcs become available. A negative reduced
cost is treated as an invariant failure. The implementation runs the full search
before updating reachable potentials; stopping at the sink without care would
leave other tentative distances unsuitable for this update.

For road graph `(V,E)`, routing work is roughly `(drivers + unique pickups) *
O((V+E) log V)`. Only one distance vector is retained at a time. With `F` feasible
pairs and `K` matches, assignment costs approximately `O(K*(F+D+R)*log(D+R))` and
uses `O(F+D+R)` residual storage. Dense candidate sets can be expensive; no spatial
pruning that might silently remove the optimal assignment is applied.

The API limits graphs to 20,000 nodes and 200,000 edges, and batches to 1,000 drivers
and 1,000 requests. Per-edge time is at most 1,000,000 ms and timestamps are bounded
at 10^12 ms. These limits keep accumulated arithmetic far below the 2^60 infinity
sentinel and bound, rather than eliminate, allocation and runtime costs. The CLI
caps JSON at 32 MiB and gives optimization a 30-second context deadline.

Graphs defensively copy input edges. `WithClosures` copies the prior snapshot and
rejects reused/non-increasing versions or unknown edge IDs. Readers of the old
snapshot observe its unchanged travel times. The returned version is provenance,
not optimistic-concurrency enforcement: an application must reject/recompute an
assignment if the authoritative graph or driver versions have changed.

The model has no fairness objective. Pickup deadlines supply per-request bounds,
but minimizing aggregate wait can prefer one distribution of delays over another.
Fairness, reservation penalties, and unserved-request costs need explicit product
objectives and new reference tests, not hidden score adjustments.
