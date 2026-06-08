package eth_test

import "math/rand/v2"

// bytePatterns returns a deterministic, broad sample of byte buffers of the
// given size for parity tests against go-ethereum's fixed-size byte types.
// Four synthetic patterns (zero, all-0xFF, all-0xAA, ascending) plus any
// caller-supplied extras (real on-chain values) plus 32 PCG-random samples.
func bytePatterns(size int, seed1, seed2 uint64, extras ...[]byte) [][]byte {
	zero := make([]byte, size)
	allFF := make([]byte, size)
	allAA := make([]byte, size)
	ascending := make([]byte, size)
	for i := range size {
		allFF[i] = 0xff
		allAA[i] = 0xaa
		ascending[i] = byte(i)
	}
	out := [][]byte{zero, allFF, allAA, ascending}
	out = append(out, extras...)

	rng := rand.New(rand.NewPCG(seed1, seed2))
	for range 32 {
		b := make([]byte, size)
		for j := range b {
			b[j] = byte(rng.Uint32())
		}
		out = append(out, b)
	}
	return out
}
