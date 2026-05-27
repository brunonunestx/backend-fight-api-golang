package pkg

import (
	"sort"
)

type Neighbor struct {
	Record   Record
	Distance uint32
}

type Record struct {
	ID     uint32
	Vector [VECTOR_SIZE]uint8
	Label  uint8
}

func FindKNN(query [VECTOR_SIZE]uint8, records []Record, k int) [5]Record {
	neighbors := make([]Neighbor, len(records))

	for i, record := range records {
		dist := CalcManhattanDistance(record.Vector, query)
		neighbors[i] = Neighbor{Record: record, Distance: dist}
	}

	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i].Distance < neighbors[j].Distance
	})

	var result [5]Record
	for i := 0; i < k; i++ {
		result[i] = neighbors[i].Record
	}

	return result
}
