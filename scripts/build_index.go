package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
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

type HKMTree struct {
	Centroids []Centroid
	Buckets   [][]Record
	Depth     uint8
	Branch    uint8
}

const (
	VECTOR_SIZE = 14
	DEPTH       = 4
	BRANCH      = 20 // 20^4 = 160000 leaf buckets
	KM_ITERS    = 15
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
		label := uint8(0)
		if ref.Label == "fraud" {
			label = 1
		}
		records[i] = Record{
			ID:     uint32(i),
			Vector: QuantitizeVector(ref.Vec),
			Label:  label,
		}
	}

	tree := BuildHKM(records, DEPTH, BRANCH)

	newFile, err := os.Create("../resources/index.ivf")
	if err != nil {
		panic(err)
	}
	defer newFile.Close()

	if err := WriteHKM(newFile, tree); err != nil {
		panic(err)
	}

	total := 0
	for _, b := range tree.Buckets {
		total += len(b)
	}
	fmt.Printf("Built HKM tree: depth=%d branch=%d nodes=%d buckets=%d records=%d\n",
		tree.Depth, tree.Branch, len(tree.Centroids), len(tree.Buckets), total)
}

func WriteHKM(f *os.File, tree HKMTree) error {
	w := bufio.NewWriter(f)

	if _, err := w.Write([]byte("HIVF")); err != nil {
		return err
	}
	if err := w.WriteByte(tree.Depth); err != nil {
		return err
	}
	if err := w.WriteByte(tree.Branch); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(tree.Centroids))); err != nil {
		return err
	}
	for _, c := range tree.Centroids {
		if err := binary.Write(w, binary.LittleEndian, c); err != nil {
			return err
		}
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(tree.Buckets))); err != nil {
		return err
	}
	for _, bucket := range tree.Buckets {
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

func intPow(base, exp int) int {
	result := 1
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

func BuildHKM(records []Record, depth, branch int) HKMTree {
	leafCount := intPow(branch, depth)
	nonLeafCount := 0
	p := 1
	for d := 0; d < depth; d++ {
		nonLeafCount += p
		p *= branch
	}
	totalNodes := nonLeafCount + leafCount

	centroids := make([]Centroid, totalNodes)
	buckets := make([][]Record, leafCount)

	buildSubtree(0, 0, records, depth, branch, centroids, buckets, nonLeafCount)

	return HKMTree{
		Centroids: centroids,
		Buckets:   buckets,
		Depth:     uint8(depth),
		Branch:    uint8(branch),
	}
}

func buildSubtree(nodeIdx, level int, records []Record, depth, branch int,
	centroids []Centroid, buckets [][]Record, nonLeafCount int) {

	if level == depth {
		buckets[nodeIdx-nonLeafCount] = records
		return
	}

	if len(records) == 0 {
		return
	}

	k := branch
	if len(records) < k {
		k = len(records)
	}

	clusterCentroids := kMeans(records, k, KM_ITERS)

	for len(clusterCentroids) < branch {
		clusterCentroids = append(clusterCentroids, clusterCentroids[rand.Intn(len(clusterCentroids))])
	}

	subClusters := make([][]Record, branch)
	for _, r := range records {
		best := 0
		bestDist := uint32(math.MaxUint32)
		for j, c := range clusterCentroids {
			d := CalcManhattanDistance(r.Vector, [VECTOR_SIZE]uint8(c))
			if d < bestDist {
				bestDist = d
				best = j
			}
		}
		subClusters[best] = append(subClusters[best], r)
	}

	for j := 0; j < branch; j++ {
		childIdx := nodeIdx*branch + j + 1
		centroids[childIdx] = clusterCentroids[j]
		buildSubtree(childIdx, level+1, subClusters[j], depth, branch, centroids, buckets, nonLeafCount)
	}
}

func kMeans(records []Record, k, maxIter int) []Centroid {
	centroids := make([]Centroid, k)
	perm := rand.Perm(len(records))
	for i := 0; i < k; i++ {
		centroids[i] = records[perm[i]].Vector
	}

	assignments := make([]int, len(records))

	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for i, r := range records {
			best := 0
			bestDist := uint32(math.MaxUint32)
			for j, c := range centroids {
				d := CalcManhattanDistance(r.Vector, [VECTOR_SIZE]uint8(c))
				if d < bestDist {
					bestDist = d
					best = j
				}
			}
			if assignments[i] != best {
				changed = true
				assignments[i] = best
			}
		}
		if !changed {
			break
		}

		sums := make([][VECTOR_SIZE]uint32, k)
		counts := make([]int, k)
		for i, r := range records {
			c := assignments[i]
			for d := 0; d < VECTOR_SIZE; d++ {
				sums[c][d] += uint32(r.Vector[d])
			}
			counts[c]++
		}
		for i := 0; i < k; i++ {
			if counts[i] > 0 {
				for d := 0; d < VECTOR_SIZE; d++ {
					centroids[i][d] = uint8(sums[i][d] / uint32(counts[i]))
				}
			}
		}
	}

	return centroids
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

func QuantitizeVector(vector []float64) [VECTOR_SIZE]uint8 {
	var quantized [VECTOR_SIZE]uint8
	for i, v := range vector {
		quantized[i] = uint8(v * 127)
	}
	return quantized
}
