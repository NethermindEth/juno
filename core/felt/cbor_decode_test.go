package felt_test

import (
	"math"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// Every build decodes both felt forms, so these tests run with and without the compactfelt tag.

// Legacy shapes stored by the Montgomery-limb encoder or accepted by its generic fallback.
var legacyDecodeCornerCases = []struct {
	name string
	data []byte
}{
	{"empty", []byte{}},
	{"nil", nil},
	{"CBOR empty array (not 4 elements)", []byte{0x80}},
	{"four zero limbs", []byte{cborArrayHeader, 0x00, 0x00, 0x00, 0x00}},
	{"four-limb array, missing one limb", []byte{cborArrayHeader, 0x00, 0x00, 0x00}},
	{"four limbs plus trailing byte", []byte{cborArrayHeader, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{"three-element array", []byte{0x83, 0x00, 0x00, 0x00}},
	{"five-element array", []byte{0x85, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{"indefinite-length array", []byte{0x9f, 0x00, 0x00, 0x00, 0x00, 0xff}},
	{"non-minimal limb header", []byte{cborArrayHeader, 0x18, 0x01, 0x00, 0x00, 0x00}},
	{"null", []byte{0xf6}},
	{"1-byte limb header, missing value byte", []byte{cborArrayHeader, 0x18}},
	{"2-byte limb header, missing value bytes", []byte{cborArrayHeader, 0x19, 0x00}},
	{"4-byte limb header, missing value bytes", []byte{cborArrayHeader, 0x1a, 0x00, 0x00, 0x00}},
	{"8-byte limb header, missing value bytes", []byte{cborArrayHeader, 0x1b, 0, 0, 0, 0, 0, 0, 0}},
	{"reserved unsigned-integer size code", []byte{cborArrayHeader, 0x1c, 0x00, 0x00, 0x00}},
	{"negative one as first limb", []byte{cborArrayHeader, 0x20, 0x00, 0x00, 0x00}},
	{"negative one as every limb", []byte{cborArrayHeader, 0x20, 0x20, 0x20, 0x20}},
	{"float32 as first limb", []byte{cborArrayHeader, 0xfa, 0x3f, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{"float64 as first limb", []byte{
		cborArrayHeader, 0xfb, 0x3f, 0xf8, 0, 0, 0, 0, 0, 0, 0x00, 0x00, 0x00,
	}},
}

// Fixed compact vectors decode in every build, whichever form the build writes.
func TestCBORDecodeCompact(t *testing.T) {
	twoToThe := func(exponent uint) felt.Felt {
		return feltFromBigInt(new(big.Int).Lsh(big.NewInt(1), exponent))
	}

	cases := []struct {
		name string
		data string
		want felt.Felt
	}{
		{"zero", "00", felt.FromUint64[felt.Felt](0)},
		{"one", "01", felt.FromUint64[felt.Felt](1)},
		{"largest uint", "17", felt.FromUint64[felt.Felt](23)},
		{"smallest byte string", "41 18", felt.FromUint64[felt.Felt](24)},
		{"largest one byte", "41 ff", felt.FromUint64[felt.Felt](math.MaxUint8)},
		{"smallest two bytes", "42 0100", felt.FromUint64[felt.Felt](math.MaxUint8 + 1)},
		{"largest uint64", "48 ffffffffffffffff", felt.FromUint64[felt.Felt](math.MaxUint64)},
		{"2^64", "49 01 0000000000000000", twoToThe(64)},
		{"largest inline length", "57 01" + strings.Repeat("00", 22), twoToThe(22 * 8)},
		{"smallest one-byte length", "58 18 01" + strings.Repeat("00", 23), twoToThe(23 * 8)},
		{"largest canonical byte string", "58 1e 80" + strings.Repeat("00", 29), twoToThe(30*8 - 1)},
		{
			"smallest Montgomery value (2^240)",
			"58 20 07dfffffd4ab5e1143ffffffffffff6f800000000001330ffffffffffd737e01",
			twoToThe(240),
		},
		{
			"largest felt (p - 1)",
			"58 20 0000000000000220000000000000000000000000000000000000000000000020",
			largestFelt(t),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var decoded felt.Felt
			require.NoError(t, decoded.UnmarshalCBOR(mustDecodeHex(t, tc.data)))
			require.Equal(t, tc.want, decoded)

			var viaFacade felt.Felt
			require.NoError(t, cbor.Unmarshal(mustDecodeHex(t, tc.data), &viaFacade))
			require.Equal(t, tc.want, viaFacade)
		})
	}
}

func TestCBORDecodeRejects(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		message string
	}{
		{"uint 24", "18 18", "uint of 24 or more"},
		{"non-minimal uint", "18 05", "uint of 24 or more"},
		{"uint64", "1b 0000000000000001", "uint of 24 or more"},
		{"empty byte string", "40", "value below 24"},
		{"byte string below 24", "41 05", "value below 24"},
		{"byte string zero", "41 00", "value below 24"},
		{"leading zero byte", "42 00ff", "leading zero"},
		{"leading zero on a 30-byte value", "58 1e 00" + strings.Repeat("ff", 29), "leading zero"},
		{"31 bytes", "58 1f 01" + strings.Repeat("00", 30), "31-byte byte string is reserved"},
		{"non-minimal length header", "58 05 0102030405", "non-minimal byte string length"},
		{"33 bytes", "58 21 01" + strings.Repeat("00", 32), "longer than 32 bytes"},
		{"two-byte length header", "59 0001 ff", "unsupported byte string length"},
		{"indefinite-length byte string", "5f 41 ff ff", "unsupported byte string length"},
		{"Montgomery modulus", "58 20" + modulusHex, "not below the field modulus"},
		{"Montgomery modulus + 1", "58 20" + modulusPlusOneHex, "not below the field modulus"},
		{"2^256 - 1", "58 20" + strings.Repeat("ff", 32), "not below the field modulus"},
		{"truncated body", "43 0102", "truncated"},
		{"missing length byte", "58", "truncated"},
		{"trailing byte after uint", "05 00", "trailing bytes"},
		{"trailing byte after byte string", "41 18 00", "trailing bytes"},
		{"negative integer", "20", "decoding legacy felt"},
		{"text string", "61 31", "decoding legacy felt"},
		{"tagged byte string", "c2 41 18", "decoding legacy felt"},
		{"true", "f5", "decoding legacy felt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sentinel := felt.FromUint64[felt.Felt](7)
			value := sentinel
			require.ErrorContains(t, value.UnmarshalCBOR(mustDecodeHex(t, tc.data)), tc.message)
			require.Equal(t, sentinel, value, "a rejected input must not change the value")
		})
	}
}

// Every proper prefix of a valid encoding is rejected.
func TestCBORDecodeRejectsTruncation(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	values := []felt.Felt{
		felt.FromUint64[felt.Felt](24),
		felt.FromUint64[felt.Felt](math.MaxUint64),
		largestFelt(t),
		randomFelt[felt.Felt](rng),
	}
	for _, value := range values {
		encoded, err := value.MarshalCBOR()
		require.NoError(t, err)
		for length := range encoded {
			var decoded felt.Felt
			require.Error(t, decoded.UnmarshalCBOR(encoded[:length]), "prefix % x", encoded[:length])
		}
	}
}

func TestCBORDecodeLegacy(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	values := []felt.Felt{
		felt.FromUint64[felt.Felt](0),
		felt.FromUint64[felt.Felt](1),
		felt.FromUint64[felt.Felt](23),
		felt.FromUint64[felt.Felt](24),
		felt.FromUint64[felt.Felt](math.MaxUint64),
		largestFelt(t),
	}
	for range 100 {
		values = append(values, randomFelt[felt.Felt](rng))
	}

	for _, value := range values {
		legacy := legacyCBOR(t, &value)

		var decoded felt.Felt
		require.NoError(t, decoded.UnmarshalCBOR(legacy))
		require.Equal(t, value, decoded)

		// what a legacy row decodes to writes back in this build's form and reads again
		reencoded, err := decoded.MarshalCBOR()
		require.NoError(t, err)
		var again felt.Felt
		require.NoError(t, again.UnmarshalCBOR(reencoded))
		require.Equal(t, value, again)
	}
}

// Legacy and current felts mix freely, so rows decode whichever encoder wrote them.
func TestCBORDecodeMixedFormats(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	values := []felt.Felt{
		felt.FromUint64[felt.Felt](7),
		felt.FromUint64[felt.Felt](math.MaxUint64),
		randomFelt[felt.Felt](rng),
		largestFelt(t),
	}

	encoded := []byte{0x80 | byte(2*len(values))}
	for _, value := range values {
		current, err := value.MarshalCBOR()
		require.NoError(t, err)
		encoded = append(encoded, legacyCBOR(t, &value)...)
		encoded = append(encoded, current...)
	}
	var want []felt.Felt
	for _, value := range values {
		want = append(want, value, value)
	}

	var slice felt.Slice[felt.Felt]
	require.NoError(t, slice.UnmarshalCBOR(encoded))
	require.Equal(t, want, []felt.Felt(slice))

	var plain []felt.Felt
	require.NoError(t, cbor.Unmarshal(encoded, &plain))
	require.Equal(t, want, plain)
}

// Legacy-shaped input decodes exactly like the generic decoder that wrote it.
func TestCBORDecodeLegacyCornerCases(t *testing.T) {
	for _, tc := range legacyDecodeCornerCases {
		t.Run(tc.name, func(t *testing.T) {
			requireLegacyDecodeEquivalent(t, tc.data)
		})
	}
}

func requireLegacyDecodeEquivalent(t *testing.T, data []byte) {
	t.Helper()

	generic, errGeneric := decodeLegacyGeneric(data)
	var fast felt.Felt
	errFast := fast.UnmarshalCBOR(data)
	if errGeneric != nil {
		require.Error(t, errFast, "accepted input the generic decoder rejected: % x", data)
		return
	}
	require.NoError(t, errFast, "rejected input the generic decoder accepted: % x", data)
	require.Equal(t, generic, fast, "decoded value mismatch for % x", data)
}
