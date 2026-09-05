package dispatch

import (
	"container/heap"
	"context"
	"errors"
)

const infinity int64 = 1 << 60
const maxTime int64 = 1_000_000_000_000

type Edge struct {
	ID     int
	From   int
	To     int
	Millis int64
	Closed bool
}
type arc struct {
	to   int
	cost int64
}

// Graph owns a defensive copy of its edges and is immutable after construction.
type Graph struct {
	version   uint64
	adjacency [][]arc
	edges     []Edge
}

func NewGraph(nodes int, version uint64, edges []Edge) (*Graph, error) {
	if nodes < 1 || nodes > 20000 || len(edges) > 200000 {
		return nil, errors.New("graph size outside limits")
	}
	g := &Graph{version: version, adjacency: make([][]arc, nodes), edges: append([]Edge(nil), edges...)}
	ids := map[int]bool{}
	for _, e := range edges {
		if e.ID < 0 || ids[e.ID] || e.From < 0 || e.From >= nodes || e.To < 0 || e.To >= nodes || e.Millis < 0 || e.Millis > 1_000_000 {
			return nil, errors.New("invalid or duplicate edge")
		}
		ids[e.ID] = true
		if !e.Closed {
			g.adjacency[e.From] = append(g.adjacency[e.From], arc{e.To, e.Millis})
		}
	}
	return g, nil
}
func (g *Graph) Version() uint64 { return g.version }

// WithClosures creates an independent snapshot; old routing queries keep their graph.
func (g *Graph) WithClosures(version uint64, closed map[int]bool) (*Graph, error) {
	if version <= g.version {
		return nil, errors.New("snapshot version must increase")
	}
	edges := append([]Edge(nil), g.edges...)
	seen := map[int]bool{}
	for i := range edges {
		if value, ok := closed[edges[i].ID]; ok {
			edges[i].Closed = value
			seen[edges[i].ID] = true
		}
	}
	if len(seen) != len(closed) {
		return nil, errors.New("unknown closure edge")
	}
	return NewGraph(len(g.adjacency), version, edges)
}

type item struct {
	node     int
	distance int64
}
type queue []item

func (q queue) Len() int { return len(q) }
func (q queue) Less(i, j int) bool {
	if q[i].distance == q[j].distance {
		return q[i].node < q[j].node
	}
	return q[i].distance < q[j].distance
}
func (q queue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)   { *q = append(*q, x.(item)) }
func (q *queue) Pop() any     { old := *q; x := old[len(old)-1]; *q = old[:len(old)-1]; return x }
func (g *Graph) Distances(ctx context.Context, source int) ([]int64, error) {
	if source < 0 || source >= len(g.adjacency) {
		return nil, errors.New("invalid source node")
	}
	dist := make([]int64, len(g.adjacency))
	for i := range dist {
		dist[i] = infinity
	}
	dist[source] = 0
	q := queue{{source, 0}}
	for len(q) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		u := heap.Pop(&q).(item)
		if u.distance != dist[u.node] {
			continue
		}
		for _, e := range g.adjacency[u.node] {
			next := u.distance + e.cost
			if next < dist[e.to] {
				dist[e.to] = next
				heap.Push(&q, item{e.to, next})
			}
		}
	}
	return dist, nil
}
func Reachable(distance int64) bool { return distance < infinity }
