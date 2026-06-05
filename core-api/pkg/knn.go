package pkg

import "math"

func l2F16vsF32(a [VECTOR_SIZE]uint16, b [VECTOR_SIZE]float32) float32 {
	var sum float32
	for i := 0; i < VECTOR_SIZE; i++ {
		d := f16ToF32(a[i]) - b[i]
		sum += d * d
	}
	return sum
}

type Record struct {
	ID     uint32
	Vector [VECTOR_SIZE]uint16 // float16 (IEEE 754 half-precision)
	Label  uint8
}

type Neighbor struct {
	Record   Record
	Distance float32
}

// FindKNN finds the k nearest neighbors across one or more record buckets.
// Returns the neighbors and how many slots are filled (filled <= k).
func FindKNN(query [VECTOR_SIZE]float32, k int, buckets ...[]Record) ([5]Record, int) {
	var top [5]Neighbor
	maxDist := float32(math.MaxFloat32)
	maxIdx := 0
	filled := 0

	for _, records := range buckets {
		for _, record := range records {
			dist := l2F16vsF32(record.Vector, query)
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
	return result, filled
}
