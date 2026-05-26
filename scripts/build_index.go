package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Reference struct {
	Vec   []float64 `json:"vector"`
	Label string    `json:"label"`
}

func main() {
	file, err := os.ReadFile("../resources/references.json")
	if err != nil {
		panic(err)
	}

	var references []Reference
	err = json.Unmarshal(file, &references)
	if err != nil {
		panic(err)
	}

	fmt.Println(references[0].Vec)
}
