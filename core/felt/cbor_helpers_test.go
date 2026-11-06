package felt_test

import (
	"encoding/hex"
	"math"
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

// compactVector is a felt and its compact CBOR encoding.
type compactVector struct {
	name    string
	value   felt.Felt
	encoded string
}

// compactVectors covers each compact size boundary; every build decodes them.
func compactVectors(t testing.TB) []compactVector {
	t.Helper()
	twoToThe := func(exponent uint) felt.Felt {
		return feltFromBigInt(new(big.Int).Lsh(big.NewInt(1), exponent))
	}
	return []compactVector{
		{"zero", felt.FromUint64[felt.Felt](0), "00"},
		{"one", felt.FromUint64[felt.Felt](1), "01"},
		{"largest uint", felt.FromUint64[felt.Felt](23), "17"},
		{"smallest byte string", felt.FromUint64[felt.Felt](24), "41 18"},
		{"largest one byte", felt.FromUint64[felt.Felt](math.MaxUint8), "41 ff"},
		{"smallest two bytes", felt.FromUint64[felt.Felt](math.MaxUint8 + 1), "42 0100"},
		{"largest uint64", felt.FromUint64[felt.Felt](math.MaxUint64), "48 ffffffffffffffff"},
		{"2^64", twoToThe(64), "49 01 0000000000000000"},
		{"largest inline length", twoToThe(22 * 8), "57 01" + strings.Repeat("00", 22)},
		{"smallest one-byte length", twoToThe(23 * 8), "58 18 01" + strings.Repeat("00", 23)},
		{"largest canonical byte string", twoToThe(30*8 - 1), "58 1e 80" + strings.Repeat("00", 29)},
		{
			"smallest Montgomery value (2^240)",
			twoToThe(240),
			"58 20 07dfffffd4ab5e1143ffffffffffff6f800000000001330ffffffffffd737e01",
		},
		{
			"2^248",
			twoToThe(248),
			"58 20 07ffffd4ab5e0098ffffffffffff6f800000000001330ffffffffffd737e0005",
		},
		{
			"largest felt (p - 1)",
			largestFelt(t),
			"58 20 0000000000000220000000000000000000000000000000000000000000000020",
		},
		{
			"modulus reduces to zero",
			feltFromBigInt(new(big.Int).SetBytes(mustDecodeHex(t, modulusHex))),
			"00",
		},
	}
}
