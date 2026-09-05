package dispatch

import (
	"context"
	"encoding/json"
	"math/rand"
	"sync"
	"testing"
)

func TestBatchAvoidsGreedyDeadEnd(t *testing.T) {
	edges := []Edge{}
	for i := 0; i < 4; i++ {
		edges = append(edges, Edge{ID: 2 * i, From: i, To: i + 1, Millis: 1}, Edge{ID: 2*i + 1, From: i + 1, To: i, Millis: 1})
	}
	g, _ := NewGraph(5, 1, edges)
	d := []Driver{{"d1", 0, 0}, {"d2", 4, 0}}
	r := []Request{{"r1", 1, 2, 0, 10}, {"r2", 0, 2, 0, 1}}
	a, err := Solve(context.Background(), g, 0, d, r)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Greedy(context.Background(), g, 0, d, r)
	if len(a.Matches) != 2 || a.TotalWait != 3 || len(b.Matches) != 1 {
		t.Fatalf("optimal=%+v greedy=%+v", a, b)
	}
}
func TestExhaustiveAssignmentOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	for trial := 0; trial < 400; trial++ {
		dn, rn := 1+rng.Intn(5), 1+rng.Intn(5)
		d := make([]Driver, dn)
		r := make([]Request, rn)
		edges := []Edge{}
		costs := make([][]int64, dn)
		for j := range r {
			r[j] = Request{ID: string(rune('a' + j)), Pickup: dn + j, Dropoff: dn + j, LatestPickup: int64(5 + rng.Intn(40))}
		}
		for i := range d {
			d[i] = Driver{ID: string(rune('A' + i)), Node: i}
			costs[i] = make([]int64, rn)
			for j := range r {
				costs[i][j] = infinity
				if rng.Intn(4) != 0 {
					cost := int64(rng.Intn(50))
					edges = append(edges, Edge{ID: len(edges), From: i, To: dn + j, Millis: cost})
					if cost <= r[j].LatestPickup {
						costs[i][j] = cost
					}
				}
			}
		}
		bestCount, bestCost := -1, infinity
		var enumerate func(int, uint, int, int64)
		enumerate = func(j int, used uint, count int, cost int64) {
			if j == rn {
				if count > bestCount || (count == bestCount && cost < bestCost) {
					bestCount, bestCost = count, cost
				}
				return
			}
			enumerate(j+1, used, count, cost)
			for i := range d {
				if used&(1<<i) == 0 && costs[i][j] < infinity {
					enumerate(j+1, used|(1<<i), count+1, cost+costs[i][j])
				}
			}
		}
		enumerate(0, 0, 0, 0)
		g, _ := NewGraph(dn+rn, 9, edges)
		got, err := Solve(context.Background(), g, 0, d, r)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Matches) != bestCount || got.TotalWait != bestCost {
			t.Fatalf("trial %d got %+v want count=%d cost=%d", trial, got, bestCount, bestCost)
		}
		// Caller order is not a hidden tie breaker.
		for i, j := 0, len(d)-1; i < j; i, j = i+1, j-1 {
			d[i], d[j] = d[j], d[i]
		}
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		again, err := Solve(context.Background(), g, 0, d, r)
		if err != nil {
			t.Fatal(err)
		}
		x, _ := json.Marshal(got)
		y, _ := json.Marshal(again)
		if string(x) != string(y) {
			t.Fatal("input-order-dependent result")
		}
	}
}
func TestDirectedRoutingAgainstFloydWarshall(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	for trial := 0; trial < 100; trial++ {
		const n = 8
		matrix := [n][n]int64{}
		edges := []Edge{}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				matrix[i][j] = infinity
				if i == j {
					matrix[i][j] = 0
				} else if rng.Intn(3) == 0 {
					w := int64(rng.Intn(20))
					matrix[i][j] = w
					edges = append(edges, Edge{ID: len(edges), From: i, To: j, Millis: w})
				}
			}
		}
		for k := 0; k < n; k++ {
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					matrix[i][j] = min(matrix[i][j], matrix[i][k]+matrix[k][j])
				}
			}
		}
		g, _ := NewGraph(n, 1, edges)
		for i := 0; i < n; i++ {
			dist, err := g.Distances(context.Background(), i)
			if err != nil {
				t.Fatal(err)
			}
			for j := 0; j < n; j++ {
				if dist[j] != matrix[i][j] {
					t.Fatal("routing oracle mismatch")
				}
			}
		}
	}
}
func TestImmutableSnapshotsAndConcurrency(t *testing.T) {
	edges := []Edge{{ID: 1, From: 0, To: 1, Millis: 5}}
	g, _ := NewGraph(2, 1, edges)
	edges[0].Millis = 999
	closed, err := g.WithClosures(2, map[int]bool{1: true})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, _ := g.Distances(context.Background(), 0)
			b, _ := closed.Distances(context.Background(), 0)
			if a[1] != 5 || Reachable(b[1]) {
				t.Error("snapshot mutated")
			}
		}()
	}
	wg.Wait()
	if _, err := g.WithClosures(1, nil); err == nil {
		t.Fatal("version reuse")
	}
	if _, err := g.WithClosures(3, map[int]bool{99: true}); err == nil {
		t.Fatal("unknown edge")
	}
}
func TestAvailabilityDeadlinesAndUnreachableTrips(t *testing.T) {
	g, _ := NewGraph(3, 1, []Edge{{ID: 1, From: 0, To: 1, Millis: 5}})
	r := []Request{{ID: "expired", Pickup: 1, Dropoff: 1, RequestedAt: 0, LatestPickup: 4}, {ID: "unreachable", Pickup: 1, Dropoff: 2, LatestPickup: 100}}
	got, err := Solve(context.Background(), g, 0, []Driver{{"driver", 0, 0}}, r)
	if err != nil || len(got.Matches) != 0 {
		t.Fatal(got, err)
	}
	r = []Request{{ID: "wait", Pickup: 1, Dropoff: 1, RequestedAt: 0, LatestPickup: 15}}
	got, err = Solve(context.Background(), g, 3, []Driver{{"driver", 0, 10}}, r)
	if err != nil || got.Matches[0].PickupAt != 15 {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Solve(ctx, g, 0, nil, r); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := Solve(context.Background(), g, 0, []Driver{{"same", 0, 0}, {"same", 1, 0}}, nil); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err := NewGraph(2, 0, []Edge{{ID: 1, From: 0, To: 1, Millis: -1}}); err == nil {
		t.Fatal("negative weight accepted")
	}
}
