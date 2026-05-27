package pkg

type Record struct {
	ID     uint32
	Vector [VECTOR_SIZE]uint8
	Label  uint8
}

type Neighbor struct {
	Record   Record
	Distance uint32
}

// FindKNN finds the k nearest neighbors across one or more record buckets.
// Accepts buckets directly to avoid concatenating slices before calling.
func FindKNN(query [VECTOR_SIZE]uint8, k int, buckets ...[]Record) [5]Record {
	var top [5]Neighbor
	maxDist := ^uint32(0)
	maxIdx := 0
	filled := 0

	for _, records := range buckets {
		for _, record := range records {
			dist := CalcManhattanDistance(record.Vector, query)
			if filled < k {
				top[filled] = Neighbor{record, dist}
				filled++
				if filled == k {
					maxDist = 0
					for i := 0; i < k; i++ {
						if top[i].Distance > maxDist {
							maxDist = top[i].Distance
							maxIdx = i
						}
					}
				}
			} else if dist < maxDist {
				top[maxIdx] = Neighbor{record, dist}
				maxDist = 0
				for i := 0; i < k; i++ {
					if top[i].Distance > maxDist {
						maxDist = top[i].Distance
						maxIdx = i
					}
				}
			}
		}
	}

	var result [5]Record
	for i := 0; i < filled; i++ {
		result[i] = top[i].Record
	}
	return result
}
