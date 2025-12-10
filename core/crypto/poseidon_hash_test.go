package crypto_test

import (
	"fmt"
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

// go test -bench=. -run=^# -cpu=1,2,4,8,16
func BenchmarkPoseidonArray(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, i := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", i), func(b *testing.B) {
			randomFeltSls := genRandomFeltSls(b, i)
			var f felt.Felt
			b.ResetTimer()
			for n := range b.N {
				f = crypto.PoseidonArray(randomFeltSls[n]...)
			}
			benchHashR = f
		})
	}
}

func BenchmarkPoseidon(b *testing.B) {
	randFelts := genRandomFeltPairs(b)

	var f felt.Felt
	b.ResetTimer()
	for n := range b.N {
		f = crypto.Poseidon(randFelts[n][0], randFelts[n][1])
	}
	benchHashR = f
}
