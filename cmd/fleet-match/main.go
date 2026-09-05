package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yafi-s/fleet-match/dispatch"
	"io"
	"os"
	"time"
)

type input struct {
	Nodes    int
	Version  uint64
	Edges    []dispatch.Edge
	Now      int64
	Drivers  []dispatch.Driver
	Requests []dispatch.Request
}

func run() error {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, (32<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 32<<20 {
		return errors.New("input exceeds 32 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var in input
	if err := decoder.Decode(&in); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON or input exceeds limit")
	}
	g, err := dispatch.NewGraph(in.Nodes, in.Version, in.Edges)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := dispatch.Solve(ctx, g, in.Now, in.Drivers, in.Requests)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
