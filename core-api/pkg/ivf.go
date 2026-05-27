package pkg

import (
	"bufio"
	"encoding/binary"
	"os"
)

type Centroid [VECTOR_SIZE]uint8

type IVFIndex struct {
	Centroids []Centroid
	Clusters  map[uint32][]Record
}

func ReadIVF(path string) (IVFIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return IVFIndex{}, err
	}
	defer f.Close()

	r := bufio.NewReader(f)

	var nCentroids, nRecords uint32

	if err := binary.Read(r, binary.LittleEndian, &nCentroids); err != nil {
		return IVFIndex{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &nRecords); err != nil {
		return IVFIndex{}, err
	}

	centroids := make([]Centroid, nCentroids)
	for i := range centroids {
		if err := binary.Read(r, binary.LittleEndian, &centroids[i]); err != nil {
			return IVFIndex{}, err
		}
	}

	clusters := make(map[uint32][]Record)
	for range nRecords {
		var record Record
		if err := binary.Read(r, binary.LittleEndian, &record.ID); err != nil {
			return IVFIndex{}, err
		}
		if err := binary.Read(r, binary.LittleEndian, &record.Vector); err != nil {
			return IVFIndex{}, err
		}
		if err := binary.Read(r, binary.LittleEndian, &record.Label); err != nil {
			return IVFIndex{}, err
		}

		clusterID := AssignToCluster(record.Vector, centroids)
		clusters[clusterID] = append(clusters[clusterID], record)
	}

	return IVFIndex{
		Centroids: centroids,
		Clusters:  clusters,
	}, nil
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
