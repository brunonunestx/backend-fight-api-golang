package pkg

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"syscall"
)

type Centroid [VECTOR_SIZE]uint8

type HKMTree struct {
	Centroids []Centroid
	Buckets   [][]Record
	Depth     uint8
	Branch    uint8
}

func ReadHKM(path string) (HKMTree, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return HKMTree{}, nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return HKMTree{}, nil, err
	}

	data, err := syscall.Mmap(
		int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ, syscall.MAP_SHARED,
	)
	if err != nil {
		return HKMTree{}, nil, err
	}

	cleanup := func() { syscall.Munmap(data) }

	pos := 0

	if string(data[pos:pos+4]) != "HIVF" {
		cleanup()
		return HKMTree{}, nil, fmt.Errorf("invalid index file: expected HIVF magic")
	}
	pos += 4

	depth := data[pos]
	pos++
	branch := data[pos]
	pos++

	nNodes := binary.LittleEndian.Uint32(data[pos:])
	pos += 4

	centroids := make([]Centroid, nNodes)
	for i := range centroids {
		copy(centroids[i][:], data[pos:pos+VECTOR_SIZE])
		pos += VECTOR_SIZE
	}

	nBuckets := binary.LittleEndian.Uint32(data[pos:])
	pos += 4

	buckets := make([][]Record, nBuckets)
	for i := range buckets {
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
		buckets[i] = records
	}

	return HKMTree{Centroids: centroids, Buckets: buckets, Depth: depth, Branch: branch}, cleanup, nil
}

type beamCandidate struct {
	nodeIdx int
	dist    uint32
}

// AssignToClusterBeam traverses the HKM tree keeping the top `beam` candidates
// at each level, returning `beam` bucket IDs at the end.
// beam=1 is greedy (fastest); beam=2-3 improves recall at ~2-3x cost.
func AssignToClusterBeam(vector [VECTOR_SIZE]uint8, tree HKMTree, beam int) []uint32 {
	depth := int(tree.Depth)
	branch := int(tree.Branch)

	current := []beamCandidate{{nodeIdx: 0}}

	for level := 0; level < depth; level++ {
		next := make([]beamCandidate, 0, len(current)*branch)
		for _, c := range current {
			firstChild := c.nodeIdx*branch + 1
			for j := 0; j < branch; j++ {
				childIdx := firstChild + j
				d := CalcManhattanDistance(vector, [VECTOR_SIZE]uint8(tree.Centroids[childIdx]))
				next = append(next, beamCandidate{childIdx, d})
			}
		}
		sort.Slice(next, func(i, j int) bool {
			return next[i].dist < next[j].dist
		})
		if len(next) > beam {
			next = next[:beam]
		}
		current = next
	}

	nonLeafCount := 0
	p := 1
	for d := 0; d < depth; d++ {
		nonLeafCount += p
		p *= branch
	}

	bucketIDs := make([]uint32, len(current))
	for i, c := range current {
		bucketIDs[i] = uint32(c.nodeIdx - nonLeafCount)
	}
	return bucketIDs
}
