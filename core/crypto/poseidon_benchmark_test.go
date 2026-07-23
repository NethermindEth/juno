package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
)

func BenchmarkPoseidonArray(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, n := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", n), func(b *testing.B) {
			feltsArray := genRandomFelts(b, n)

			for b.Loop() {
				crypto.PoseidonArray(feltsArray)
			}
		})
	}
}

func BenchmarkPoseidonElems(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, n := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", n), func(b *testing.B) {
			elems := genRandomFeltsPtr(b, n)
			for b.Loop() {
				crypto.PoseidonElems(elems...)
			}
		})
	}
}

func BenchmarkPoseidon(b *testing.B) {
	in := genRandomFelts(b, 2)

	for b.Loop() {
		crypto.Poseidon(&in[0], &in[1])
	}
}

// BenchmarkPoseidonDigest locks in the allocation count of the streaming
// digest path (Update + Finish), which struct/array hashing relies on.
// 40 felts exercises ~20 Hades permutations.
func BenchmarkPoseidonDigest(b *testing.B) {
	elems := genRandomFeltsPtr(b, 40)

	b.ReportAllocs()
	for b.Loop() {
		var digest crypto.PoseidonDigest
		digest.Update(elems...)
		digest.Finish()
	}
}
