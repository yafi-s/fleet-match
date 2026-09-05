package dispatch

import (
	"container/heap"
	"context"
	"errors"
	"sort"
)

type Driver struct {
	ID          string
	Node        int
	AvailableAt int64
}
type Request struct {
	ID           string
	Pickup       int
	Dropoff      int
	RequestedAt  int64
	LatestPickup int64
}
type Match struct {
	Driver   string `json:"driver"`
	Request  string `json:"request"`
	PickupAt int64  `json:"pickup_at_ms"`
	FinishAt int64  `json:"finish_at_ms"`
	Wait     int64  `json:"wait_ms"`
}
type Result struct {
	GraphVersion   uint64   `json:"graph_version"`
	Matches        []Match  `json:"matches"`
	Unassigned     []string `json:"unassigned"`
	TotalWait      int64    `json:"total_wait_ms"`
	FeasiblePairs  int      `json:"feasible_pairs"`
	RoutingQueries int      `json:"routing_queries"`
}
type pair struct {
	driver, request      int
	pickup, finish, cost int64
}
type prepared struct {
	drivers  []Driver
	requests []Request
	pairs    []pair
	queries  int
}

func prepare(ctx context.Context, g *Graph, now int64, drivers []Driver, requests []Request) (prepared, error) {
	p := prepared{drivers: append([]Driver(nil), drivers...), requests: append([]Request(nil), requests...)}
	if g == nil || now < 0 || now > maxTime || len(drivers) > 1000 || len(requests) > 1000 {
		return p, errors.New("invalid batch limits")
	}
	sort.Slice(p.drivers, func(i, j int) bool { return p.drivers[i].ID < p.drivers[j].ID })
	sort.Slice(p.requests, func(i, j int) bool { return p.requests[i].ID < p.requests[j].ID })
	nodes := len(g.adjacency)
	for i, d := range p.drivers {
		if d.ID == "" || d.Node < 0 || d.Node >= nodes || d.AvailableAt < 0 || d.AvailableAt > maxTime || (i > 0 && d.ID == p.drivers[i-1].ID) {
			return p, errors.New("invalid/duplicate driver")
		}
	}
	for i, r := range p.requests {
		if r.ID == "" || r.Pickup < 0 || r.Pickup >= nodes || r.Dropoff < 0 || r.Dropoff >= nodes || r.RequestedAt < 0 || r.RequestedAt > now || r.LatestPickup < r.RequestedAt || r.LatestPickup > maxTime || (i > 0 && r.ID == p.requests[i-1].ID) {
			return p, errors.New("invalid/duplicate request")
		}
	}
	// One bounded routing vector at a time. Cache only trip durations and candidate
	// edges, not O((drivers+requests)*nodes) full distance matrices.
	trips := make([]int64, len(p.requests))
	byPickup := map[int][]int{}
	for j, r := range p.requests {
		byPickup[r.Pickup] = append(byPickup[r.Pickup], j)
	}
	for origin, indices := range byPickup {
		dist, err := g.Distances(ctx, origin)
		if err != nil {
			return p, err
		}
		p.queries++
		for _, j := range indices {
			trips[j] = dist[p.requests[j].Dropoff]
		}
	}
	for i, d := range p.drivers {
		dist, err := g.Distances(ctx, d.Node)
		if err != nil {
			return p, err
		}
		p.queries++
		for j, r := range p.requests {
			if !Reachable(dist[r.Pickup]) || !Reachable(trips[j]) {
				continue
			}
			pickup := max(now, d.AvailableAt) + dist[r.Pickup]
			if pickup > r.LatestPickup {
				continue
			}
			p.pairs = append(p.pairs, pair{i, j, pickup, pickup + trips[j], pickup - r.RequestedAt})
		}
	}
	return p, nil
}

type residual struct {
	to, reverse, capacity int
	cost                  int64
}
type network [][]residual

func (n network) add(from, to int, cost int64) {
	n[from] = append(n[from], residual{to, len(n[to]), 1, cost})
	n[to] = append(n[to], residual{from, len(n[from]) - 1, 0, -cost})
}

// Solve lexicographically maximizes matched requests, then minimizes total pickup
// wait. It does not sacrifice a feasible rider to improve the average of others.
func Solve(ctx context.Context, g *Graph, now int64, drivers []Driver, requests []Request) (Result, error) {
	p, err := prepare(ctx, g, now, drivers, requests)
	if err != nil {
		return Result{}, err
	}
	d, r := len(p.drivers), len(p.requests)
	source, sink := d+r, d+r+1
	net := make(network, d+r+2)
	for i := 0; i < d; i++ {
		net.add(source, i, 0)
	}
	for j := 0; j < r; j++ {
		net.add(d+j, sink, 0)
	}
	type edgeRef struct{ from, index int }
	refs := make([]edgeRef, len(p.pairs))
	for i, e := range p.pairs {
		refs[i] = edgeRef{e.driver, len(net[e.driver])}
		net.add(e.driver, d+e.request, e.cost)
	}
	potential := make([]int64, len(net))
	for {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		dist := make([]int64, len(net))
		prevNode, prevEdge := make([]int, len(net)), make([]int, len(net))
		for i := range dist {
			dist[i] = infinity
			prevNode[i] = -1
		}
		dist[source] = 0
		q := queue{{source, 0}}
		for len(q) > 0 {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			u := heap.Pop(&q).(item)
			if u.distance != dist[u.node] {
				continue
			}
			for i, e := range net[u.node] {
				if e.capacity == 0 {
					continue
				}
				reduced := e.cost + potential[u.node] - potential[e.to]
				if reduced < 0 {
					return Result{}, errors.New("negative reduced cost invariant")
				}
				next := u.distance + reduced
				if next < dist[e.to] {
					dist[e.to] = next
					prevNode[e.to] = u.node
					prevEdge[e.to] = i
					heap.Push(&q, item{e.to, next})
				}
			}
		}
		if prevNode[sink] < 0 {
			break
		}
		for i := range potential {
			if dist[i] < infinity {
				potential[i] += dist[i]
			}
		}
		for v := sink; v != source; {
			u, i := prevNode[v], prevEdge[v]
			back := net[u][i].reverse
			net[u][i].capacity--
			net[v][back].capacity++
			v = u
		}
	}
	chosen := []pair{}
	for i, ref := range refs {
		if net[ref.from][ref.index].capacity == 0 {
			chosen = append(chosen, p.pairs[i])
		}
	}
	return result(g, p, chosen), nil
}

// Greedy is a deterministic request-order nearest-eligible-driver baseline.
func Greedy(ctx context.Context, g *Graph, now int64, drivers []Driver, requests []Request) (Result, error) {
	p, err := prepare(ctx, g, now, drivers, requests)
	if err != nil {
		return Result{}, err
	}
	used := make([]bool, len(p.drivers))
	chosen := []pair{}
	for j := range p.requests {
		best := -1
		for i, e := range p.pairs {
			if e.request == j && !used[e.driver] && (best < 0 || e.cost < p.pairs[best].cost) {
				best = i
			}
		}
		if best >= 0 {
			chosen = append(chosen, p.pairs[best])
			used[p.pairs[best].driver] = true
		}
	}
	return result(g, p, chosen), nil
}
func result(g *Graph, p prepared, chosen []pair) Result {
	out := Result{GraphVersion: g.version, Matches: []Match{}, Unassigned: []string{}, FeasiblePairs: len(p.pairs), RoutingQueries: p.queries}
	assigned := map[int]bool{}
	for _, e := range chosen {
		out.Matches = append(out.Matches, Match{p.drivers[e.driver].ID, p.requests[e.request].ID, e.pickup, e.finish, e.cost})
		out.TotalWait += e.cost
		assigned[e.request] = true
	}
	for j, r := range p.requests {
		if !assigned[j] {
			out.Unassigned = append(out.Unassigned, r.ID)
		}
	}
	sort.Slice(out.Matches, func(i, j int) bool { return out.Matches[i].Request < out.Matches[j].Request })
	return out
}
