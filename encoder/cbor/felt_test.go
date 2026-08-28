package cbor_test

// TODO(granza): Build a pull of felts to test.
// core/felt/cbor_fastpath_test.go should test them fast vs generic,
// here should test current engine vs canonical encoder (fxmacker).

import (
	"testing"

	"github.com/NethermindEth/juno/encoder/cbor"
	fxcbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

type limbs = [cbor.Limbs]uint64

var marshalFeltCases = []struct {
	name  string
	value limbs
}{
	{"zero", limbs{}},
	{"one byte limbs", limbs{1, 2, 3, 4}},
	{"mixed widths", limbs{24, 256, 65536, 4294967296}},
	{"montgomery shaped", limbs{
		18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960,
	}},
}

// fxamacker is the source of truth. It wrote every felt on disk, so engines must agree with it.
// core/felt should not care about this assertion.
func TestMarshalFeltMatchesFxamacker(t *testing.T) {
	for _, test := range marshalFeltCases {
		t.Run(test.name, func(t *testing.T) {
			generic, err := fxcbor.Marshal(test.value)
			require.NoError(t, err)
			require.Equal(t, generic, cbor.MarshalFelt(&test.value))
		})
	}
}

func TestUnmarshalFeltMatchesFxamacker(t *testing.T) {
	for _, test := range marshalFeltCases {
		t.Run(test.name, func(t *testing.T) {
			written, err := fxcbor.Marshal(test.value)
			require.NoError(t, err)

			var back limbs
			require.True(t, cbor.UnmarshalFelt(written, &back))
			require.Equal(t, test.value, back)
		})
	}
}
