package crypto_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestPoseidon(t *testing.T) {
	left := felt.NewFromUint64[felt.Felt](1)
	right := felt.NewFromUint64[felt.Felt](2)

	hash := crypto.Poseidon[felt.Felt](left, right)
	assert.Equal(
		t,
		"0x5d44a3decb2b2e0cc71071f7b802f45dd792d064f0fc7316c46514f70f9891a",
		hash.String(),
	)
}

func TestPoseidonArray(t *testing.T) {
	for name, test := range map[string]struct {
		elems    []*felt.Felt
		expected string
	}{
		"empty array": {
			elems:    []*felt.Felt{},
			expected: "0x2272be0f580fd156823304800919530eaa97430e972d7213ee13f4fbf7a5dbc",
		},
		"odd elems": {
			elems: []*felt.Felt{
				new(felt.Felt), felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
			},
			expected: "0x7a01142da8aecae3782ba66fc3285fd02fcd2c55aa868fe50fd95c089068d16",
		},
		"even elems": {
			elems: []*felt.Felt{
				new(felt.Felt), felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2), felt.NewFromUint64[felt.Felt](3),
			},
			expected: "0x7b8f30ac298ea12d170c0873f1fa631a18c00756c6e7d1fd273b9a239d0d413",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var digest, digestWhole crypto.PoseidonDigest
			hash := crypto.PoseidonElems[felt.Felt](test.elems...)
			assert.Equal(t, test.expected, hash.String())

			// PoseidonArray (value slice) must match the pointer version exactly.
			vals := make([]felt.Felt, len(test.elems))
			for i, e := range test.elems {
				vals[i] = *e
			}
			valsHash := crypto.PoseidonArray[felt.Felt](vals)
			assert.Equal(t, test.expected, valsHash.String())

			for _, elem := range test.elems {
				digestWhole.Update(elem)
			}
			hash = digestWhole.Finish[felt.Felt]()
			assert.Equal(t, test.expected, hash.String())

			for _, elem := range test.elems {
				digest.Update(elem)
			}
			hash = digest.Finish[felt.Felt]()
			assert.Equal(t, test.expected, hash.String())
		})
	}
}

func TestPoseidonDigestUpdateArrayMatchesUpdate(t *testing.T) {
	for size := range 33 {
		vals := make([]felt.Felt, size)
		ptrs := make([]*felt.Felt, size)
		for idx := range size {
			vals[idx] = felt.Random[felt.Felt]()
			ptrs[idx] = &vals[idx]
		}

		var byArray, byElems crypto.PoseidonDigest
		byArray.UpdateArray(vals)
		for _, elem := range ptrs {
			byElems.Update(elem)
		}
		assert.Equal(t, byElems.Finish[felt.Felt](), byArray.Finish[felt.Felt]())
	}
}
