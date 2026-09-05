# Validation record

Implementation commit: `a7e0a253a064d3003307c45c8a386a5357f2129b`.

[GitHub CI run](https://github.com/yafi-s/fleet-match/actions/runs/33942495110)
completed successfully on 2026-09-05 UTC on Go 1.23.x and 1.27.x. Each job checked
formatting, ran `go vet`, ran tests under the race detector, built both commands,
executed the JSON example, and ran the three-seed comparison experiment.

Local tests passed on Apple M2 with Go 1.27.1. The dispatch package reported 94.4%
statement coverage; command entrypoints are not included in that coverage figure.
Separate local CLI checks passed for a valid example, unknown JSON fields, trailing
documents, a payload exceeding 32 MiB, and malformed JSON.

Correctness checks include an exhaustive assignment oracle for 400 small random
batches, a Floyd–Warshall routing oracle for 100 random graphs, permutation
invariance, cancellation, deadline and availability boundaries, unreachable
dropoffs, and concurrent readers of immutable closure snapshots.
