package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yafi-s/fleet-match/dispatch"
	"math/rand"
	"os"
	"time"
)

func main() {
	const width = 30
	edges := []dispatch.Edge{}
	for y := 0; y < width; y++ {
		for x := 0; x < width; x++ {
			n := y*width + x
			for _, delta := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				xx, yy := x+delta[0], y+delta[1]
				if xx >= 0 && yy >= 0 && xx < width && yy < width {
					edges = append(edges, dispatch.Edge{ID: len(edges), From: n, To: yy*width + xx, Millis: 1000})
				}
			}
		}
	}
	g, err := dispatch.NewGraph(width*width, 1, edges)
	if err != nil {
		panic(err)
	}
	for _, seed := range []int64{7, 19, 41} {
		rng := rand.New(rand.NewSource(seed))
		drivers := []dispatch.Driver{}
		requests := []dispatch.Request{}
		for i := 0; i < 150; i++ {
			drivers = append(drivers, dispatch.Driver{ID: fmt.Sprintf("d%03d", i), Node: rng.Intn(width * width)})
			requests = append(requests, dispatch.Request{ID: fmt.Sprintf("r%03d", i), Pickup: rng.Intn(width * width), Dropoff: rng.Intn(width * width), LatestPickup: int64(3000 + rng.Intn(12000))})
		}
		for _, mode := range []string{"greedy", "batch"} {
			start := time.Now()
			var result dispatch.Result
			if mode == "greedy" {
				result, err = dispatch.Greedy(context.Background(), g, 0, drivers, requests)
			} else {
				result, err = dispatch.Solve(context.Background(), g, 0, drivers, requests)
			}
			if err != nil {
				panic(err)
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"seed": seed, "mode": mode, "drivers": 150, "requests": 150, "matched": len(result.Matches), "total_wait_ms": result.TotalWait, "solve_ms": float64(time.Since(start).Microseconds()) / 1000, "feasible_pairs": result.FeasiblePairs, "routing_queries": result.RoutingQueries})
		}
	}
}
