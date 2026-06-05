package fraud

import (
	pkg "core-api/pkg"
)

const (
	VECTOR_SIZE = 14
	kNeighbors  = 5
	nprobe      = 10
)

type DetectionResult struct {
	Approved bool
	Score    float64
}

type Service struct {
	idx *pkg.IVFIndex
}

func NewService(idx *pkg.IVFIndex) *Service {
	return &Service{idx: idx}
}

func (s *Service) DetectFraudRaw(body []byte) DetectionResult {
	vec64 := FastBuildVector(body)
	var query [pkg.VECTOR_SIZE]float32
	for i, v := range vec64 {
		query[i] = float32(v)
	}

	neighbors, filled := s.idx.Search(query, kNeighbors, nprobe)

	fraudCount := 0
	for i := 0; i < filled; i++ {
		if neighbors[i].Label == 1 {
			fraudCount++
		}
	}

	score := float64(fraudCount) / float64(kNeighbors)
	return DetectionResult{
		Approved: score < 0.5,
		Score:    score,
	}
}
