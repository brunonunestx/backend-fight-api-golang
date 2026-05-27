package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Reference struct {
	Vec   []float64 `json:"vector"`
	Label string    `json:"label"`
}

type Record struct {
	ID     uint32
	Vector [VECTOR_SIZE]uint8
	Label  uint8
}

type Centroid [VECTOR_SIZE]uint8

type IVFIndex struct {
	Centroids []Centroid
	Buckets   [][]Record
}

const (
	VECTOR_SIZE = 14
	N_CLUSTERS  = 1000
)

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

	records := make([]Record, len(references))

	for i, ref := range references {
		quantitizedVector := QuantitizeVector(ref.Vec)
		label := uint8(0)
		if ref.Label == "fraud" {
			label = 1
		}

		records[i] = Record{
			ID:     uint32(i),
			Vector: quantitizedVector,
			Label:  label,
		}
	}

	index := BuildIVF(records, N_CLUSTERS)

	newFile, err := os.Create("../resources/index.ivf")
	if err != nil {
		panic(err)
	}

	defer newFile.Close()

	if err := WriteIVF(newFile, index); err != nil {
		panic(err)
	}

	total := 0
	for _, b := range index.Buckets {
		total += len(b)
	}
	fmt.Printf("Built IVF index with %d centroids, %d clusters, %d records\n", len(index.Centroids), len(index.Buckets), total)
}

func WriteIVF(f *os.File, index IVFIndex) error {
	w := bufio.NewWriter(f)

	if err := binary.Write(w, binary.LittleEndian, uint32(len(index.Centroids))); err != nil {
		return err
	}

	for _, c := range index.Centroids {
		if err := binary.Write(w, binary.LittleEndian, c); err != nil {
			return err
		}
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(index.Buckets))); err != nil {
		return err
	}

	for _, bucket := range index.Buckets {
		if err := binary.Write(w, binary.LittleEndian, uint32(len(bucket))); err != nil {
			return err
		}
		for _, r := range bucket {
			if err := binary.Write(w, binary.LittleEndian, r.ID); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, r.Vector); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, r.Label); err != nil {
				return err
			}
		}
	}

	return w.Flush()
}

func BuildIVF(records []Record, nClusters int) IVFIndex {
	centroids := make([]Centroid, nClusters)
	for i := 0; i < nClusters; i++ {
		centroids[i] = records[i].Vector
	}

	buckets := make([][]Record, nClusters)
	for _, record := range records {
		bestCluster := 0
		bestDist := uint32(math.MaxUint32)
		for i, centroid := range centroids {
			dist := CalcManhattanDistance(record.Vector, centroid)
			if dist < bestDist {
				bestDist = dist
				bestCluster = i
			}
		}
		buckets[bestCluster] = append(buckets[bestCluster], record)
	}

	return IVFIndex{
		Centroids: centroids,
		Buckets:   buckets,
	}
}

func CalcManhattanDistance(a, b [14]uint8) uint32 {
	var sum uint32

	for i := 0; i < 14; i++ {
		diff := int32(a[i]) - int32(b[i])

		if diff < 0 {
			diff = -diff
		}

		sum += uint32(diff)
	}

	return sum
}

func QuantitizeVector(vector []float64) [VECTOR_SIZE]uint8 {
	var quantized [VECTOR_SIZE]uint8
	for i, v := range vector {
		quantized[i] = uint8(v * 127)
	}
	return quantized
}
