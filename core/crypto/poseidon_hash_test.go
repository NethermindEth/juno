package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestPoseidonCrossCheckRust pins our output against Rust starknet-crypto
// (poseidon_hash), an independent implementation, over diverse inputs including
// edge cases (0, 1, q-1). Guards the cgo/Rust path and the pure-Go fallback.
func TestPoseidonCrossCheckRust(t *testing.T) {
	const qm1 = "0x800000000000010ffffffffffffffffffffffffffffffffffffffffffffffff" // q-1
	cases := [][3]string{
		{"0x0", "0x0", "0x293d3e8a80f400daaaffdd5932e2bcc8814bab8f414a75dcacf87318f8b14c5"},
		{"0x0", "0x1", "0x5134197931125e849424475aa20cd6ca0ce8603b79177c3f76e2119c8f98c53"},
		{"0x1", "0x2", "0x5d44a3decb2b2e0cc71071f7b802f45dd792d064f0fc7316c46514f70f9891a"},
		{
			"0x800000000000011000000000000000000000000000000000000000000000000", "0x1",
			"0x2df67287e559fa5c7ec5ab725cea8eeb6464edb23906dfb1c7d217153aa3e51",
		},
		{qm1, qm1, "0x7d3fa153b4cdbed8f36f43d3a2c62d270f5d24370cb268dc9e1f91e139cf56e"},
		{"0xdeadbeef", "0xcafe", "0x2655e71a2ccab97e524aa047808576c5068e08e9f6541d3262f28226c07691a"},
		{
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x3",
			"0x6fba2b31920e16603460d0e839400d4d0c69dbf2e61bb85acd1489c542c9856",
		},
		{
			"0x123456789abcdef", "0xfedcba987654321",
			"0x34b061abd52c7e99e84b0f2477fc2c08b5fd8e19c64f8c702442c8014bca1a",
		},
		{"0x2", qm1, "0x61ca48c5d1a85f69e57de3b3c0c5b273f2f8d262dd4d6a12264777df6f6a778"},
		{
			"0x5d44a3decb2b2e0cc71071f7b802f45dd792d064f0fc7316c46514f70f9891a",
			"0x7a01142da8aecae3782ba66fc3285fd02fcd2c55aa868fe50fd95c089068d16",
			"0x71fe1a9080c6646e4f03da5dcd92e3b751dfddf2111257a3bf89ccd80984bd",
		},
	}
	for _, c := range cases {
		x, err := new(felt.Felt).SetString(c[0])
		require.NoError(t, err)
		y, err := new(felt.Felt).SetString(c[1])
		require.NoError(t, err)
		got := crypto.Poseidon(x, y)
		assert.Equal(t, c[2], got.String(), "Poseidon(%s, %s)", c[0], c[1])
	}
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
			hash := crypto.PoseidonArray(test.elems...)
			assert.Equal(t, test.expected, hash.String())
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

func BenchmarkPoseidonArray(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, n := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", n), func(b *testing.B) {
			elems := genRandomFelts(b, n)
			var f felt.Felt
			for b.Loop() {
				f = crypto.PoseidonArray(elems...)
			}
			benchHashR = f
		})
	}
}

func BenchmarkPoseidon(b *testing.B) {
	in := genRandomFelts(b, 2)

	var f felt.Felt
	for b.Loop() {
		f = crypto.Poseidon(in[0], in[1])
	}
	benchHashR = f
}
