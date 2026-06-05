package main

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"

	server "core-api/internal/server"
)

func main() {
	// With 0.40 CPUs per container, running more than 1 OS thread causes the
	// scheduler and GC to compete for the same fraction of a CPU. GOMAXPROCS=1
	// ensures a single goroutine queue, reducing preemption and context switching.
	runtime.GOMAXPROCS(1)

	// Run GC less frequently — heap can grow to 4x the live set before collecting.
	// Paired with GOMEMLIMIT to avoid OOM in 150MB containers.
	debug.SetGCPercent(400)
	debug.SetMemoryLimit(150 * 1024 * 1024)

	server := server.New(":8080", nil)
	fmt.Println("Starting server on :8080")

	if err := server.Start(); err != nil {
		panic(err)
	}

	defer server.Stop(context.Background())
}
