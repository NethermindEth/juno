package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
)

// go test -bench=. -run=^# -cpu=1,2,4,8,16
func BenchmarkPedersenArray(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, n := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", n), func(b *testing.B) {
			feltsArray := genRandomFelts(b, n)
			for b.Loop() {
				crypto.PedersenArray(feltsArray)
			}
		})
	}
}

func BenchmarkPedersenElems(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, n := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", n), func(b *testing.B) {
			elems := genRandomFeltsPtr(b, n)
			for b.Loop() {
				crypto.PedersenElems(elems...)
			}
		})
	}
}

func BenchmarkPedersen(b *testing.B) {
	felts := genRandomFeltsPtr(b, 2)
	for b.Loop() {
		crypto.Pedersen(felts[0], felts[1])
	}
}
