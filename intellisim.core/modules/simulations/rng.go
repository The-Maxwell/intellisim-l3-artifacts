package simulations

import (
	"encoding/binary"
	"hash/fnv"
	"math/rand"
)

// RunRNG derives a deterministic *rand.Rand from a seed and a sequence number.
// The same (seed, runSeq) pair always yields an RNG that reproduces an
// identical draw sequence.
//
// The Generator (generator.go) always calls this with runSeq FIXED at 1: since
// every generation run (create or regenerate) draws/stores a fresh sim.Seed
// before running (ISS-03), a full-scope regeneration that happens to draw seed X
// is byte-identical to CREATING a new simulation with seed X — the stored seed
// is always a faithful, reproducible description of the current world. runSeq
// remains a parameter (rather than being hardcoded away) because it is also used
// directly by placement/sampler helpers and their unit tests, which vary it to
// prove RNG-stream independence between draws.
func RunRNG(seed, runSeq uint64) *rand.Rand {
	h := fnv.New64a()
	var b [16]byte
	binary.LittleEndian.PutUint64(b[:8], seed)
	binary.LittleEndian.PutUint64(b[8:], runSeq)
	h.Write(b[:])
	return rand.New(rand.NewSource(int64(h.Sum64())))
}
