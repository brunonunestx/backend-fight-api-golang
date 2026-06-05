package pkg

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

const hivfMagic = "HIV2"

type IVFIndex struct {
	nlist1      int
	nlist2      int
	l1Centroids [][VECTOR_SIZE]float32
	l2Centroids [][VECTOR_SIZE]float32 // flat: [l1*nlist2 + l2]
	buckets     [][]Record             // flat: [l1*nlist2 + l2]
}

// LoadIVF streams the index file without loading the full 99 MB into memory.
// All records are written into a single backing array to avoid allocator fragmentation.
func LoadIVF(path string) (*IVFIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	// 33 bytes per record on disk (4 id + 28 vec f16 + 1 label); slight overestimate is fine.
	estimatedRecords := int(fi.Size()) / 33

	r := bufio.NewReaderSize(f, 8*1024*1024)

	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("ivf: read magic: %w", err)
	}
	if string(magic[:]) != hivfMagic {
		return nil, fmt.Errorf("ivf: invalid header (got %q)", string(magic[:]))
	}

	nlist1, err := readU32(r)
	if err != nil {
		return nil, err
	}
	nlist2, err := readU32(r)
	if err != nil {
		return nil, err
	}
	vectorSize, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if int(vectorSize) != VECTOR_SIZE {
		return nil, fmt.Errorf("ivf: vector size mismatch: got %d, want %d", vectorSize, VECTOR_SIZE)
	}

	total := int(nlist1) * int(nlist2)
	idx := &IVFIndex{
		nlist1:      int(nlist1),
		nlist2:      int(nlist2),
		l1Centroids: make([][VECTOR_SIZE]float32, nlist1),
		l2Centroids: make([][VECTOR_SIZE]float32, total),
		buckets:     make([][]Record, total),
	}

	// Single contiguous allocation for all records — eliminates per-bucket allocator fragmentation.
	pool := make([]Record, 0, estimatedRecords)

	var raw [VECTOR_SIZE * 2]byte // reused buffer for reading one vector

	for c := range idx.l1Centroids {
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			return nil, err
		}
		for d := 0; d < VECTOR_SIZE; d++ {
			idx.l1Centroids[c][d] = f16ToF32(binary.LittleEndian.Uint16(raw[d*2:]))
		}
	}

	for l1 := 0; l1 < int(nlist1); l1++ {
		base := l1 * int(nlist2)
		for l2 := 0; l2 < int(nlist2); l2++ {
			if _, err := io.ReadFull(r, raw[:]); err != nil {
				return nil, err
			}
			for d := 0; d < VECTOR_SIZE; d++ {
				idx.l2Centroids[base+l2][d] = f16ToF32(binary.LittleEndian.Uint16(raw[d*2:]))
			}
		}
		for l2 := 0; l2 < int(nlist2); l2++ {
			count, err := readU32(r)
			if err != nil {
				return nil, err
			}
			start := len(pool)
			for i := 0; i < int(count); i++ {
				id, err := readU32(r)
				if err != nil {
					return nil, err
				}
				if _, err := io.ReadFull(r, raw[:]); err != nil {
					return nil, err
				}
				var vec [VECTOR_SIZE]uint16
				for d := 0; d < VECTOR_SIZE; d++ {
					vec[d] = binary.LittleEndian.Uint16(raw[d*2:])
				}
				label, err := r.ReadByte()
				if err != nil {
					return nil, err
				}
				pool = append(pool, Record{ID: id, Vector: vec, Label: label})
			}
			idx.buckets[base+l2] = pool[start:len(pool):len(pool)]
		}
	}

	return idx, nil
}

func readU32(r io.Reader) (uint32, error) {
	var b [4]byte
	_, err := io.ReadFull(r, b[:])
	return binary.LittleEndian.Uint32(b[:]), err
}

// Search finds the k nearest neighbors using 2-level hierarchical lookup.
// nprobe = number of L2 leaf buckets to visit.
// Internally scans all nlist1 L1 centroids (cheap), then all L2 centroids within
// the top nprobeL1 = nprobe/nlist2+2 L1 clusters, then visits the top nprobe L2 buckets.
func (idx *IVFIndex) Search(query [VECTOR_SIZE]float32, k, nprobe int) ([5]Record, int) {
	nprobeL1 := nprobe/idx.nlist2 + 2
	if nprobeL1 > idx.nlist1 {
		nprobeL1 = idx.nlist1
	}

	topL1 := idx.topNCentroids(query, idx.l1Centroids, nprobeL1)

	l2Entries := make([]ivfEntry, 0, nprobeL1*idx.nlist2)
	for _, l1 := range topL1 {
		base := l1 * idx.nlist2
		for l2 := 0; l2 < idx.nlist2; l2++ {
			d := centroidDist(query, &idx.l2Centroids[base+l2])
			l2Entries = append(l2Entries, ivfEntry{base + l2, d})
		}
	}

	topL2 := topNEntries(l2Entries, nprobe)
	buckets := make([][]Record, len(topL2))
	for i, e := range topL2 {
		buckets[i] = idx.buckets[e.idx]
	}
	return FindKNN(query, k, buckets...)
}

type ivfEntry struct {
	idx  int
	dist float32
}

func (idx *IVFIndex) topNCentroids(query [VECTOR_SIZE]float32, centroids [][VECTOR_SIZE]float32, n int) []int {
	top := make([]ivfEntry, 0, n)
	maxDist := float32(math.MaxFloat32)
	maxPos := 0

	for c := range centroids {
		d := centroidDist(query, &centroids[c])
		if len(top) < n {
			top = append(top, ivfEntry{c, d})
			if len(top) == n {
				maxDist, maxPos = worstIVFEntry(top)
			}
		} else if d < maxDist {
			top[maxPos] = ivfEntry{c, d}
			maxDist, maxPos = worstIVFEntry(top)
		}
	}

	result := make([]int, len(top))
	for i, e := range top {
		result[i] = e.idx
	}
	return result
}

func topNEntries(entries []ivfEntry, n int) []ivfEntry {
	if n >= len(entries) {
		return entries
	}
	top := make([]ivfEntry, 0, n)
	maxDist := float32(math.MaxFloat32)
	maxPos := 0

	for _, e := range entries {
		if len(top) < n {
			top = append(top, e)
			if len(top) == n {
				maxDist, maxPos = worstIVFEntry(top)
			}
		} else if e.dist < maxDist {
			top[maxPos] = e
			maxDist, maxPos = worstIVFEntry(top)
		}
	}
	return top
}

func worstIVFEntry(top []ivfEntry) (float32, int) {
	maxDist, maxPos := float32(0), 0
	for i, e := range top {
		if e.dist > maxDist {
			maxDist = e.dist
			maxPos = i
		}
	}
	return maxDist, maxPos
}

func centroidDist(query [VECTOR_SIZE]float32, centroid *[VECTOR_SIZE]float32) float32 {
	var sum float32
	for i := 0; i < VECTOR_SIZE; i++ {
		d := query[i] - centroid[i]
		sum += d * d
	}
	return sum
}

func f16ToF32(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := int((h >> 10) & 0x1F)
	mant := uint32(h & 0x3FF)

	if exp == 0 {
		if mant == 0 {
			return math.Float32frombits(sign)
		}
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		exp++
		mant &= 0x3FF
	} else if exp == 0x1F {
		return math.Float32frombits(sign | 0x7F800000 | mant<<13)
	}

	return math.Float32frombits(sign | uint32(exp+127-15)<<23 | mant<<13)
}
