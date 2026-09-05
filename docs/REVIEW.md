# Review exercises

Walk through the five-node example by hand. Show how a residual reverse edge can
undo an initially attractive pairing. Then explain why selecting the cheapest
available edge repeatedly does not solve the full assignment problem.

Derive the objective ordering: served count first, total wait second. Construct a
case where this increases total wait versus greedy and explain why that is not a
solver regression. Construct another where worst-case individual wait increases.

Read the independent exhaustive oracle and the Floyd–Warshall routing oracle.
Identify which bugs each can catch and which large-instance behaviors neither
establishes. Inspect why randomizing input entity order must preserve the result.

A substantive extension is versioned assignment application: atomically reserve
drivers only if both graph and driver-state revisions still match. Model duplicate
submissions and partial failures before adding a network API.
