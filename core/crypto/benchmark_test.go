package crypto_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

func genRandomFelts(b *testing.B, n int) []felt.Felt {
	b.Helper()
	felts := make([]felt.Felt, n)
	for i := range n {
		felts[i] = felt.Random[felt.Felt]()
	}
	return felts
}

func genRandomFeltsPtr(b *testing.B, n int) []*felt.Felt {
	b.Helper()
	felts := make([]*felt.Felt, n)
	for i := range n {
		felts[i] = felt.NewRandom[felt.Felt]()
	}
	return felts
}
