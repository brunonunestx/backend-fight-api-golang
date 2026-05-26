package main

import (
	"context"
	"fmt"

	server "core-api/internal/server"
)

func main() {
	server := server.New(":8080", nil)
	fmt.Println("Starting server on :8080")

	if err := server.Start(); err != nil {
		panic(err)
	}

	defer server.Stop(context.Background())
}
