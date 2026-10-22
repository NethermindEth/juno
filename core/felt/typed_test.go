package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// Every felt type must encode exactly like Felt in every format, so retyping never changes bytes.
func TestTypedEncodesLikeFelt(t *testing.T) {
	values := map[string]felt.Felt{
		"zero":       {},
		"one":        felt.One,
		"small":      felt.FromUint64[felt.Felt](23),
		"word":       felt.FromUint64[felt.Felt](1 << 63),
		"full width": felt.Random[felt.Felt](),
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			assertSameEncodings[felt.Address](t, value)
			assertSameEncodings[felt.Hash](t, value)
			assertSameEncodings[felt.ClassHash](t, value)
			assertSameEncodings[felt.SierraClassHash](t, value)
			assertSameEncodings[felt.CasmClassHash](t, value)
			assertSameEncodings[felt.TransactionHash](t, value)
			assertSameEncodings[felt.StateRootHash](t, value)
		})
	}
}

// encodings is what every felt type must answer identically to Felt.
type encodings interface {
	MarshalText() ([]byte, error)
	String() string
	Marshal() []byte
	Bytes() [felt.Bytes]byte
}

// assertSameEncodings is instantiated with the felt type itself; its pointer type is inferred.
func assertSameEncodings[T felt.FeltLike, PT interface {
	*T
	encodings
}](t *testing.T, value felt.Felt) {
	t.Helper()
	typed := T(value)

	feltCBOR, err := cbor.Marshal(&value)
	require.NoError(t, err)
	typedCBOR, err := cbor.Marshal(PT(&typed))
	require.NoError(t, err)
	require.Equal(t, feltCBOR, typedCBOR, "cbor")

	var decoded T
	require.NoError(t, cbor.Unmarshal(feltCBOR, PT(&decoded)))
	require.Equal(t, typed, decoded, "cbor round trip")

	feltText, err := value.MarshalText()
	require.NoError(t, err)
	typedText, err := PT(&typed).MarshalText()
	require.NoError(t, err)
	require.Equal(t, feltText, typedText, "text")
	require.Equal(t, value.String(), PT(&typed).String(), "string")

	require.Equal(t, value.Marshal(), PT(&typed).Marshal(), "raw bytes")
	require.Equal(t, value.Bytes(), PT(&typed).Bytes(), "bytes")
}
