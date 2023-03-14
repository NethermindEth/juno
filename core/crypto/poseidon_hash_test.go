package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

var f *felt.Felt

func TestPoseidon(t *testing.T) {
	left := new(felt.Felt).SetUint64(1)
	right := new(felt.Felt).SetUint64(2)

	assert.Equal(t, "0x5d44a3decb2b2e0cc71071f7b802f45dd792d064f0fc7316c46514f70f9891a",
		crypto.Poseidon(left, right).String())
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
			assert.Equal(t, test.expected, crypto.PoseidonArray(test.elems...).String())
		})
	}
}

// go test -bench=. -run=^# -cpu=1,2,4,8,16
func BenchmarkPoseidonArray(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}
	createRandomFelts := func(n int) []*felt.Felt {
		var felts []*felt.Felt
		for i := 0; i < n; i++ {
			f, err := new(felt.Felt).SetRandom()
			if err != nil {
				b.Fatalf("error while generating random felt: %x", err)
			}
			felts = append(felts, f)
		}
		return felts
	}

	for _, i := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", i), func(b *testing.B) {
			randomFelts := createRandomFelts(i)
			for n := 0; n < b.N; n++ {
				f = crypto.PoseidonArray(randomFelts...)
			}
			feltBench = f
		})
	}
}

func BenchmarkPoseidon(b *testing.B) {
	e0, err := new(felt.Felt).SetString("0x3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb")
	if err != nil {
		b.Fatalf("Error occured %s", err)
	}

	e1, err := new(felt.Felt).SetString("0x208a0a10250e382e1e4bbe2880906c2791bf6275695e02fbbc6aeff9cd8b31a")
	if err != nil {
		b.Fatalf("Error occured %s", err)
	}

	for n := 0; n < b.N; n++ {
		f = crypto.Poseidon(e0, e1)
	}
	feltBench = f
}
