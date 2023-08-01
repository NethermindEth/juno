package p2p

import (
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_BitsetSerialization(t *testing.T) {
	bset := bitset.New(0)
	assertBitsetTheSame(t, bset)

	bset = bitset.New(10)
	assertBitsetTheSame(t, bset)

	bset = bitset.New(10)
	bset.Set(1)
	bset.Set(6)
	assertBitsetTheSame(t, bset)

	bset = bitset.New(200)
	bset.Set(1)
	bset.Set(6)
	bset.Set(80)
	bset.Set(85)
	assertBitsetTheSame(t, bset)
}

func assertBitsetTheSame(t *testing.T, bset *bitset.BitSet) {
	assert.True(t, protoToBitset(bitsetToProto(bset)).Equal(bset))
}
