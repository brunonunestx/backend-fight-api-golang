package pkg

const VECTOR_SIZE = 14

func QuantitizeVector(vector []float64) [VECTOR_SIZE]uint8 {
	var quantized [VECTOR_SIZE]uint8
	for i, v := range vector {
		quantized[i] = uint8(v * 127)
	}
	return quantized
}

func CalcManhattanDistance(a, b [VECTOR_SIZE]uint8) uint32 {
	var sum uint32
	for i := 0; i < VECTOR_SIZE; i++ {
		diff := int32(a[i]) - int32(b[i])
		if diff < 0 {
			diff = -diff
		}
		sum += uint32(diff)
	}
	return sum
}
