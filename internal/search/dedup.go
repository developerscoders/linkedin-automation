package search

import (
	"hash/fnv"
	"math"
)

type BloomFilter struct {
	bitset []bool
	k      uint
	m      uint
}

func NewBloomFilter(n uint, p float64) *BloomFilter {
	m := uint(math.Ceil(float64(n) * math.Log(p) / math.Log(1.0/math.Pow(2.0, math.Log(2.0)))))
	k := uint(math.Round(math.Log(2.0) * float64(m) / float64(n)))

	return &BloomFilter{
		bitset: make([]bool, m),
		k:      k,
		m:      m,
	}
}

func (bf *BloomFilter) Add(data []byte) {
	h := fnv.New64a()
	h.Write(data)
	hash1 := h.Sum64()
	
	h2 := fnv.New64()
	h2.Write(data)
	hash2 := h2.Sum64()

	for i := uint(0); i < bf.k; i++ {
		idx := (hash1 + uint64(i)*hash2) % uint64(bf.m)
		bf.bitset[idx] = true
	}
}

func (bf *BloomFilter) Contains(data []byte) bool {
	h := fnv.New64a()
	h.Write(data)
	hash1 := h.Sum64()

	h2 := fnv.New64()
	h2.Write(data)
	hash2 := h2.Sum64()

	for i := uint(0); i < bf.k; i++ {
		idx := (hash1 + uint64(i)*hash2) % uint64(bf.m)
		if !bf.bitset[idx] {
			return false
		}
	}
	return true
}
