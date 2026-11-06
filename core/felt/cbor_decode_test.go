package felt_test

import (
	"math"
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
	for _, tc := range compactVectors(t) {
		t.Run(tc.name, func(t *testing.T) {
			var decoded felt.Felt
			require.NoError(t, decoded.UnmarshalCBOR(mustDecodeHex(t, tc.encoded)))
			require.Equal(t, tc.value, decoded)

			var viaFacade felt.Felt
			require.NoError(t, cbor.Unmarshal(mustDecodeHex(t, tc.encoded), &viaFacade))
			require.Equal(t, tc.value, viaFacade)
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
		{
			"non-minimal length header",
			"58 05 0102030405",
			"non-minimal byte string length: length 5 must be in the header byte",
		},
		{"33 bytes", "58 21 01" + strings.Repeat("00", 32), "longer than 32 bytes: got 33"},
		{"two-byte length header", "59 0001 ff", "unsupported byte string length"},
		{"indefinite-length byte string", "5f 41 ff ff", "unsupported byte string length"},
		{"Montgomery modulus", "58 20" + modulusHex, "not below the field modulus"},
		{"Montgomery modulus + 1", "58 20" + modulusPlusOneHex, "not below the field modulus"},
		{"2^256 - 1", "58 20" + strings.Repeat("ff", 32), "not below the field modulus"},
		{"truncated body", "43 0102", "truncated byte string: header declares 3 bytes, got 2"},
		{"missing length byte", "58", "truncated byte string: missing the length byte"},
		{
			"trailing byte after uint",
			"05 00",
			"trailing bytes: input has 2 bytes but the felt encoding ends after 1",
		},
		{
			"trailing byte after byte string",
			"41 18 00",
			"trailing bytes: input has 3 bytes but the felt encoding ends after 2",
		},
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
