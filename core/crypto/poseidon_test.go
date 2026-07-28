package crypto_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestPoseidon(t *testing.T) {
	left := new(felt.Felt).SetUint64(1)
	right := new(felt.Felt).SetUint64(2)

	hash := crypto.Poseidon(left, right)
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
				new(felt.Felt), new(felt.Felt).SetUint64(1),
				new(felt.Felt).SetUint64(2),
			},
			expected: "0x7a01142da8aecae3782ba66fc3285fd02fcd2c55aa868fe50fd95c089068d16",
		},
		"even elems": {
			elems: []*felt.Felt{
				new(felt.Felt), new(felt.Felt).SetUint64(1),
				new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3),
			},
			expected: "0x7b8f30ac298ea12d170c0873f1fa631a18c00756c6e7d1fd273b9a239d0d413",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var digest, digestWhole crypto.PoseidonDigest
			hash := crypto.PoseidonElems(test.elems...)
			assert.Equal(t, test.expected, hash.String())

			// PoseidonArray (value slice) must match the pointer version exactly.
			vals := make([]felt.Felt, len(test.elems))
			for i, e := range test.elems {
				vals[i] = *e
			}
			valsHash := crypto.PoseidonArray(vals)
			assert.Equal(t, test.expected, valsHash.String())

			hash = digestWhole.Update(test.elems...).Finish()
			assert.Equal(t, test.expected, hash.String())

			for _, elem := range test.elems {
				digest.Update(elem)
			}
			hash = digest.Finish()
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
		byElems.Update(ptrs...)
		assert.Equal(t, byElems.Finish(), byArray.Finish())
	}
}

// Test vectors from https://github.com/starkware-industries/poseidon
func TestHadesPermutation(t *testing.T) {
	state := [3]felt.Felt{}
	crypto.HadesPermutation(&state)

	want := [3]string{
		"3446325744004048536138401612021367625846492093718951375866996507163446763827",
		"1590252087433376791875644726012779423683501236913937337746052470473806035332",
		"867921192302518434283879514999422690776342565400001269945778456016268852423",
	}
	for i, w := range want {
		assert.Equal(t, w, state[i].Text(10))
	}
}
