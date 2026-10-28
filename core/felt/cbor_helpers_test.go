package felt_test

import (
	"encoding/hex"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/require"
)

// modulusHex is the field modulus p as 32 big-endian bytes.
const modulusHex = "0800000000000011000000000000000000000000000000000000000000000001"

// cborArrayHeader is the CBOR header byte for an array of 4 items, the legacy felt shape.
const cborArrayHeader = 0x84

// legacyCBOR returns the Montgomery-limb encoding the legacy encoder writes for value.
func legacyCBOR(t testing.TB, value *felt.Felt) []byte {
	t.Helper()
	encoded, err := cbor.Marshal((*fp.Element)(value))
	require.NoError(t, err)
	require.Equal(t, byte(cborArrayHeader), encoded[0])
	return encoded
}

// randomFelt returns a reduced felt drawn from rng.
func randomFelt[F felt.FeltLike](rng *rand.Rand) F {
	var data [32]byte
	_, _ = rng.Read(data[:])
	return felt.FromBytes[F](data[:])
}

func mustDecodeHex(t testing.TB, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(strings.ReplaceAll(value, " ", ""))
	require.NoError(t, err)
	return data
}

// largestFeltHex is p - 1, the largest canonical felt, as 32 big-endian bytes.
const largestFeltHex = "0800000000000011000000000000000000000000000000000000000000000000"

// modulusPlusOneHex is p + 1 as 32 big-endian bytes.
const modulusPlusOneHex = "0800000000000011000000000000000000000000000000000000000000000002"

// largestFelt returns p - 1.
func largestFelt(t testing.TB) felt.Felt {
	t.Helper()
	var decoded felt.Felt
	require.NoError(t, decoded.SetBytesCanonical(mustDecodeHex(t, largestFeltHex)))
	return decoded
}

// decodeLegacyGeneric decodes data the way the generic CBOR decoder reads a plain limb array.
func decodeLegacyGeneric(data []byte) (felt.Felt, error) {
	var generic felt.Felt
	err := cbor.Unmarshal(data, (*fp.Element)(&generic))
	return generic, err
}

func feltFromBigInt(n *big.Int) felt.Felt {
	var value felt.Felt
	value.SetBigInt(n)
	return value
}
