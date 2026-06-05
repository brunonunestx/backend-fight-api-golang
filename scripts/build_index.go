package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	VectorSize = 14
	NList1     = 128 // L1 clusters
	NList2     = 64  // L2 clusters per L1; total = 128*64 = 8192
	KMeansIter = 30
)

type rawEntry struct {
	Vector [VectorSize]float64 `json:"vector"`
	Label  string              `json:"label"`
}

type Record struct {
	ID     uint32
	Vector [VectorSize]float32
	Label  uint8
}

func main() {
	start := time.Now()

	fmt.Println("[1/5] loading records...")
	records := loadRecords("resources/references.json")
	fmt.Printf("      %d records (%.1fs)\n", len(records), time.Since(start).Seconds())

	fmt.Printf("[2/5] L1 k-means: %d clusters...\n", NList1)
	t := time.Now()
	rng := rand.New(rand.NewSource(42))
	l1Centroids, l1Assign := kMeansParallel(records, NList1, KMeansIter, rng)
	fmt.Printf("      done (%.1fs)\n", time.Since(t).Seconds())

	fmt.Println("[3/5] partitioning into L1 buckets...")
	l1Buckets := make([][]Record, NList1)
	for i, r := range records {
		c := l1Assign[i]
		l1Buckets[c] = append(l1Buckets[c], r)
	}
	minL1, maxL1 := len(records), 0
	for _, b := range l1Buckets {
		if len(b) < minL1 {
			minL1 = len(b)
		}
		if len(b) > maxL1 {
			maxL1 = len(b)
		}
	}
	fmt.Printf("      L1 sizes: min=%d avg=%d max=%d\n", minL1, len(records)/NList1, maxL1)

	fmt.Printf("[4/5] L2 k-means: %d sub-clusters per L1 (%d total), parallel...\n", NList2, NList1*NList2)
	t = time.Now()
	l2Centroids := make([][VectorSize]float32, NList1*NList2)
	buckets := make([][]Record, NList1*NList2)

	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0

	for l1 := 0; l1 < NList1; l1++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(l1 int, seed int64) {
			defer func() { <-sem; wg.Done() }()
			subset := l1Buckets[l1]
			if len(subset) == 0 {
				return
			}
			k := NList2
			if len(subset) < k {
				k = len(subset)
			}
			localRng := rand.New(rand.NewSource(seed))
			centroids2, assign2 := kMeansSeq(subset, k, KMeansIter, localRng)

			for len(centroids2) < NList2 {
				src := centroids2[localRng.Intn(len(centroids2))]
				perturbed := src
				perturbed[0] = math.Float32frombits(math.Float32bits(perturbed[0]) ^ 1)
				centroids2 = append(centroids2, perturbed)
			}

			subBuckets := make([][]Record, NList2)
			for i, r := range subset {
				subBuckets[assign2[i]] = append(subBuckets[assign2[i]], r)
			}

			base := l1 * NList2
			mu.Lock()
			copy(l2Centroids[base:base+NList2], centroids2)
			copy(buckets[base:base+NList2], subBuckets)
			done++
			if done%32 == 0 || done == NList1 {
				fmt.Printf("  %d/%d L1 clusters done\n", done, NList1)
			}
			mu.Unlock()
		}(l1, rng.Int63())
	}
	wg.Wait()
	fmt.Printf("      done (%.1fs)\n", time.Since(t).Seconds())

	fmt.Println("[5/5] writing index...")
	t = time.Now()
	if err := saveHIVF(l1Centroids, l2Centroids, buckets, "resources/index.ivf"); err != nil {
		panic(err)
	}
	fmt.Printf("      done (%.1fs)\n", time.Since(t).Seconds())

	total, minSz, maxSz, empty := 0, math.MaxInt32, 0, 0
	for _, b := range buckets {
		n := len(b)
		total += n
		if n < minSz {
			minSz = n
		}
		if n > maxSz {
			maxSz = n
		}
		if n == 0 {
			empty++
		}
	}
	fmt.Printf("\nbuckets=%d records=%d avg=%.1f min=%d max=%d empty=%d\n",
		len(buckets), total, float64(total)/float64(len(buckets)), minSz, maxSz, empty)
	fmt.Printf("total: %.1fs\n", time.Since(start).Seconds())
}

func loadRecords(path string) []Record {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var raw []rawEntry
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		panic(err)
	}

	records := make([]Record, len(raw))
	for i, r := range raw {
		var label uint8
		if r.Label == "fraud" {
			label = 1
		}
		var vec [VectorSize]float32
		for j, v := range r.Vector {
			vec[j] = float32(v)
		}
		records[i] = Record{ID: uint32(i), Vector: vec, Label: label}
	}
	return records
}

// kMeansParallel runs k-means with k-means++ init and parallel assignment.
func kMeansParallel(records []Record, k, maxIter int, rng *rand.Rand) ([][VectorSize]float32, []int) {
	n := len(records)
	numCPU := runtime.NumCPU()
	centroids := kmeansPlusPlus(records, k, rng)
	assignments := make([]int, n)
	newAssignments := make([]int, n)
	chunkSize := (n + numCPU - 1) / numCPU
	changed := make([]bool, numCPU)

	for iter := 0; iter < maxIter; iter++ {
		t := time.Now()
		for i := range changed {
			changed[i] = false
		}

		var wg sync.WaitGroup
		for w := 0; w < numCPU; w++ {
			lo := w * chunkSize
			hi := lo + chunkSize
			if hi > n {
				hi = n
			}
			wg.Add(1)
			go func(w, lo, hi int) {
				defer wg.Done()
				for i := lo; i < hi; i++ {
					c := nearestInSlice(&records[i].Vector, centroids)
					newAssignments[i] = c
					if c != assignments[i] {
						changed[w] = true
					}
				}
			}(w, lo, hi)
		}
		wg.Wait()
		copy(assignments, newAssignments)

		anyChanged := false
		for _, c := range changed {
			if c {
				anyChanged = true
				break
			}
		}

		sums := make([][VectorSize]float64, k)
		counts := make([]int, k)
		for i, r := range records {
			c := assignments[i]
			counts[c]++
			for d := 0; d < VectorSize; d++ {
				sums[c][d] += float64(r.Vector[d])
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] == 0 {
				centroids[c] = records[rng.Intn(n)].Vector
				continue
			}
			for d := 0; d < VectorSize; d++ {
				centroids[c][d] = float32(sums[c][d] / float64(counts[c]))
			}
		}

		fmt.Printf("  iter %2d/%d (%v)\n", iter+1, maxIter, time.Since(t))
		if !anyChanged {
			fmt.Printf("  converged at iter %d\n", iter+1)
			break
		}
	}

	return centroids, assignments
}

// kMeansSeq runs sequential k-means (for L2 sub-clusters, already parallelized at the L1 level).
func kMeansSeq(records []Record, k, maxIter int, rng *rand.Rand) ([][VectorSize]float32, []int) {
	n := len(records)
	centroids := kmeansPlusPlus(records, k, rng)
	assignments := make([]int, n)

	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for i, r := range records {
			c := nearestInSlice(&r.Vector, centroids)
			if assignments[i] != c {
				changed = true
				assignments[i] = c
			}
		}
		if !changed {
			break
		}

		sums := make([][VectorSize]float64, k)
		counts := make([]int, k)
		for i, r := range records {
			c := assignments[i]
			counts[c]++
			for d := 0; d < VectorSize; d++ {
				sums[c][d] += float64(r.Vector[d])
			}
		}

		maxCount, maxC := 0, 0
		for c, count := range counts {
			if count > maxCount {
				maxCount = count
				maxC = c
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				for d := 0; d < VectorSize; d++ {
					centroids[c][d] = float32(sums[c][d] / float64(counts[c]))
				}
			} else {
				centroids[c] = centroids[maxC]
				centroids[c][0] = math.Float32frombits(math.Float32bits(centroids[c][0]) ^ 1)
			}
		}
	}

	return centroids, assignments
}

func kmeansPlusPlus(records []Record, k int, rng *rand.Rand) [][VectorSize]float32 {
	centroids := make([][VectorSize]float32, 0, k)
	centroids = append(centroids, records[rng.Intn(len(records))].Vector)

	minDists := make([]float64, len(records))
	var totalDist float64
	for i, r := range records {
		d := float64(l2Dist(&r.Vector, &centroids[0]))
		minDists[i] = d
		totalDist += d
	}

	for len(centroids) < k {
		if totalDist == 0 {
			centroids = append(centroids, records[rng.Intn(len(records))].Vector)
			continue
		}
		target := rng.Float64() * totalDist
		cumulative := 0.0
		chosen := len(records) - 1
		for i, d := range minDists {
			cumulative += d
			if cumulative >= target {
				chosen = i
				break
			}
		}
		newC := records[chosen].Vector
		centroids = append(centroids, newC)
		totalDist = 0
		for i, r := range records {
			d := float64(l2Dist(&r.Vector, &newC))
			if d < minDists[i] {
				minDists[i] = d
			}
			totalDist += minDists[i]
		}
	}

	return centroids
}

func nearestInSlice(v *[VectorSize]float32, centroids [][VectorSize]float32) int {
	best := 0
	bestDist := float32(math.MaxFloat32)
	for c := range centroids {
		if d := l2Dist(v, &centroids[c]); d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func l2Dist(a, b *[VectorSize]float32) float32 {
	var sum float32
	for i := 0; i < VectorSize; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

// Binary format "HIV2":
//
//	[4]  "HIV2"
//	[4]  nlist1 uint32
//	[4]  nlist2 uint32
//	[4]  vector_size uint32
//	nlist1 × (vector_size × uint16)   — L1 centroids
//	per L1:
//	  nlist2 × (vector_size × uint16) — L2 centroids
//	  per L2:
//	    [4] count uint32
//	    per record: [4] id, vector_size×uint16, [1] label
func saveHIVF(
	l1Centroids [][VectorSize]float32,
	l2Centroids [][VectorSize]float32,
	buckets [][]Record,
	path string,
) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 16*1024*1024)
	le := binary.LittleEndian
	var u32 [4]byte
	var u16 [2]byte

	bw.WriteString("HIV2")
	le.PutUint32(u32[:], uint32(NList1))
	bw.Write(u32[:])
	le.PutUint32(u32[:], uint32(NList2))
	bw.Write(u32[:])
	le.PutUint32(u32[:], uint32(VectorSize))
	bw.Write(u32[:])

	for _, c := range l1Centroids {
		for d := 0; d < VectorSize; d++ {
			le.PutUint16(u16[:], f32ToF16(c[d]))
			bw.Write(u16[:])
		}
	}

	for l1 := 0; l1 < NList1; l1++ {
		base := l1 * NList2
		for l2 := 0; l2 < NList2; l2++ {
			for d := 0; d < VectorSize; d++ {
				le.PutUint16(u16[:], f32ToF16(l2Centroids[base+l2][d]))
				bw.Write(u16[:])
			}
		}
		for l2 := 0; l2 < NList2; l2++ {
			bucket := buckets[base+l2]
			le.PutUint32(u32[:], uint32(len(bucket)))
			bw.Write(u32[:])
			for _, r := range bucket {
				le.PutUint32(u32[:], r.ID)
				bw.Write(u32[:])
				for d := 0; d < VectorSize; d++ {
					le.PutUint16(u16[:], f32ToF16(r.Vector[d]))
					bw.Write(u16[:])
				}
				bw.WriteByte(r.Label)
			}
		}
	}

	return bw.Flush()
}

func f32ToF16(f float32) uint16 {
	b := math.Float32bits(f)
	sign := uint16((b >> 31) & 1)
	exp := int((b >> 23) & 0xFF)
	mant := b & 0x7FFFFF

	if exp == 0xFF {
		if mant != 0 {
			return sign<<15 | 0x7E00
		}
		return sign<<15 | 0x7C00
	}

	exp -= 127
	if exp > 15 {
		return sign<<15 | 0x7C00
	}
	if exp < -14 {
		if exp < -24 {
			return sign << 15
		}
		mant |= 1 << 23
		shift := uint(-1 - exp)
		mant >>= shift
		return sign<<15 | uint16(mant>>13)
	}

	return sign<<15 | uint16(exp+15)<<10 | uint16(mant>>13)
}
