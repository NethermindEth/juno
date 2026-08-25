package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	fxcbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// selfDecodingFelt carries the same hook a real felt does.
type selfDecodingFelt [4]uint64

var (
	_ cbor.SelfEncoder = (*selfDecodingFelt)(nil)
	_ cbor.SelfDecoder = (*selfDecodingFelt)(nil)
)

func (f *selfDecodingFelt) MarshalCBOR() ([]byte, error) {
	return cbor.MarshalFelt(f)
}

func (f *selfDecodingFelt) UnmarshalCBOR(data []byte) error {
	return cbor.UnmarshalFelt(data, f)
}

// A four-limb array, but the header says its size is arbitrary.
var indefiniteLengthFelt = []byte{0x9f, 0x01, 0x02, 0x03, 0x04, 0xff}

func TestUnmarshalFeltFallbackDoesNotRecurse(t *testing.T) {
	var value selfDecodingFelt
	require.NoError(t, value.UnmarshalCBOR(indefiniteLengthFelt))
	require.Equal(t, selfDecodingFelt{1, 2, 3, 4}, value)
}

func TestUnmarshalFeltNullLeavesTargetUntouched(t *testing.T) {
	value := selfDecodingFelt{9, 9, 9, 9}
	require.NoError(t, value.UnmarshalCBOR([]byte{0xf6}))
	require.Equal(t, selfDecodingFelt{9, 9, 9, 9}, value)
}

var marshalFeltCases = []struct {
	name  string
	value selfDecodingFelt
}{
	{"zero", selfDecodingFelt{}},
	{"one byte limbs", selfDecodingFelt{1, 2, 3, 4}},
	{"mixed widths", selfDecodingFelt{24, 256, 65536, 4294967296}},
	{"montgomery shaped", selfDecodingFelt{
		18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960,
	}},
}

// fxamacker is the source of truth. It wrote every felt that is on disk today.
// A new engine should agree with it.
func TestMarshalFeltMatchesFxamacker(t *testing.T) {
	for _, test := range marshalFeltCases {
		t.Run(test.name, func(t *testing.T) {
			fast, err := test.value.MarshalCBOR()
			require.NoError(t, err)

			generic, err := fxcbor.Marshal([4]uint64(test.value))
			require.NoError(t, err)
			require.Equal(t, generic, fast)
		})
	}
}

func TestMarshalFeltMatchesEngine(t *testing.T) {
	for _, test := range marshalFeltCases {
		t.Run(test.name, func(t *testing.T) {
			fast, err := test.value.MarshalCBOR()
			require.NoError(t, err)

			generic, err := cbor.Marshal([4]uint64(test.value))
			require.NoError(t, err)
			require.Equal(t, generic, fast)

			var back selfDecodingFelt
			require.NoError(t, back.UnmarshalCBOR(fast))
			require.Equal(t, test.value, back)
		})
	}
}
