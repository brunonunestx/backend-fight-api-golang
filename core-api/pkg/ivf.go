package pkg

import (
	"encoding/binary"
	"os"
	"syscall"
)

type Centroid [VECTOR_SIZE]uint8

type IVFIndex struct {
	Centroids []Centroid
	Clusters  map[uint32][]Record
}

func ReadIVF(path string) (IVFIndex, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return IVFIndex{}, nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return IVFIndex{}, nil, err
	}

	data, err := syscall.Mmap(
		int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ, syscall.MAP_SHARED,
	)
	if err != nil {
		return IVFIndex{}, nil, err
	}

	cleanup := func() { syscall.Munmap(data) }

	pos := 0
	nCentroids := binary.LittleEndian.Uint32(data[pos:])
	pos += 4

	centroids := make([]Centroid, nCentroids)
	for i := range centroids {
		copy(centroids[i][:], data[pos:pos+VECTOR_SIZE])
		pos += VECTOR_SIZE
	}

	nClusters := binary.LittleEndian.Uint32(data[pos:])
	pos += 4

	clusters := make(map[uint32][]Record, nClusters)
	for i := range nClusters {
		clusterSize := binary.LittleEndian.Uint32(data[pos:])
		pos += 4

		records := make([]Record, clusterSize)
		for j := range records {
			records[j].ID = binary.LittleEndian.Uint32(data[pos:])
			pos += 4
			copy(records[j].Vector[:], data[pos:pos+VECTOR_SIZE])
			pos += VECTOR_SIZE
			records[j].Label = data[pos]
			pos++
		}
		clusters[i] = records
	}

	return IVFIndex{Centroids: centroids, Clusters: clusters}, cleanup, nil
}

func AssignToCluster(vector [VECTOR_SIZE]uint8, centroids []Centroid) uint32 {
	best := uint32(0)
	bestDist := CalcManhattanDistance(vector, [VECTOR_SIZE]uint8(centroids[0]))

	for i := 1; i < len(centroids); i++ {
		dist := CalcManhattanDistance(vector, [VECTOR_SIZE]uint8(centroids[i]))
		if dist < bestDist {
			bestDist = dist
			best = uint32(i)
		}
	}

	return best
}
