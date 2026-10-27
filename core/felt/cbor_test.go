//go:build compactfelt

package felt_test

import (
	"bytes"
	"encoding/hex"
	"math"
	"math/big"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/require"
)

// largestFeltHex is p - 1, the largest canonical felt, as 32 big-endian bytes.
const largestFeltHex = "0800000000000011000000000000000000000000000000000000000000000000"

// modulusPlusOneHex is p + 1 as 32 big-endian bytes.
const modulusPlusOneHex = "0800000000000011000000000000000000000000000000000000000000000002"

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

// largestFelt returns p - 1.
func largestFelt(t testing.TB) felt.Felt {
	t.Helper()
	var decoded felt.Felt
	require.NoError(t, decoded.SetBytesCanonical(mustDecodeHex(t, largestFeltHex)))
	return decoded
}

// decodeLegacyGeneric decodes data the way fxamacker reads a plain limb array.
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

// requireCompactRoundTrip checks that value encodes to its expected length and decodes back.
func requireCompactRoundTrip(t *testing.T, value *felt.Felt) []byte {
	t.Helper()

	encoded, err := value.MarshalCBOR()
	require.NoError(t, err)
	require.Equal(t, expectedCBORLen(value), len(encoded), "encoding % x", encoded)

	if len(encoded) == 34 {
		require.Equal(t, montgomeryHex(value), hex.EncodeToString(encoded[2:]), "Montgomery bytes")
	}

	var decoded felt.Felt
	require.NoError(t, decoded.UnmarshalCBOR(encoded))
	require.Equal(t, *value, decoded)
	return encoded
}

// expectedCBORLen is the encoding length derived from the big-endian value.
func expectedCBORLen(value *felt.Felt) int {
	raw := value.Bytes()
	trimmed := bytes.TrimLeft(raw[:], "\x00")
	switch {
	case len(trimmed) == 0 || (len(trimmed) == 1 && trimmed[0] < 24):
		return 1
	case len(trimmed) < 24:
		return 1 + len(trimmed)
	case len(trimmed) <= 30:
		return 2 + len(trimmed)
	default:
		return 2 + 32
	}
}

// montgomeryHex returns value times 2^256 modulo p as 32 big-endian hex bytes, using big.Int.
func montgomeryHex(value *felt.Felt) string {
	montgomery := value.BigInt(new(big.Int))
	montgomery.Lsh(montgomery, 256).Mod(montgomery, fp.Modulus())
	return hex.EncodeToString(montgomery.FillBytes(make([]byte, 32)))
}

func TestCBOREncoding(t *testing.T) {
	twoToThe := func(exponent uint) felt.Felt {
		return feltFromBigInt(new(big.Int).Lsh(big.NewInt(1), exponent))
	}

	cases := []struct {
		name  string
		value felt.Felt
		want  string
	}{
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

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := requireCompactRoundTrip(t, &tc.value)
			require.Equal(t, mustDecodeHex(t, tc.want), encoded)

			viaFacade, err := cbor.Marshal(&tc.value)
			require.NoError(t, err)
			require.Equal(t, encoded, viaFacade)

			var viaFacadeDecoded felt.Felt
			require.NoError(t, cbor.Unmarshal(encoded, &viaFacadeDecoded))
			require.Equal(t, tc.value, viaFacadeDecoded)
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

		// legacy -> compact -> compact is stable
		compact := requireCompactRoundTrip(t, &decoded)
		require.NotEqual(t, byte(cborArrayHeader), compact[0])
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

func TestCBORAliases(t *testing.T) {
	t.Run("Felt", testCBORAlias[felt.Felt])
	t.Run("Address", testCBORAlias[felt.Address])
	t.Run("Hash", testCBORAlias[felt.Hash])
	t.Run("ClassHash", testCBORAlias[felt.ClassHash])
	t.Run("SierraClassHash", testCBORAlias[felt.SierraClassHash])
	t.Run("CasmClassHash", testCBORAlias[felt.CasmClassHash])
	t.Run("TransactionHash", testCBORAlias[felt.TransactionHash])
	t.Run("StateRootHash", testCBORAlias[felt.StateRootHash])
}

// testCBORAlias checks that an alias encodes like Felt as a value, pointer, slice element and
// map key, and still decodes legacy limbs.
func testCBORAlias[F felt.FeltLike](t *testing.T) {
	type record struct {
		Value   F
		Pointer *F
		Nil     *F
		Values  []F
		Keys    map[F]F
	}

	rng := rand.New(rand.NewSource(1))
	value := randomFelt[F](rng)
	small := felt.FromUint64[F](5)
	original := record{
		Value:   value,
		Pointer: &small,
		Values:  []F{small, value},
		Keys:    map[F]F{value: small},
	}

	encoded, err := cbor.Marshal(original)
	require.NoError(t, err)

	asFelt := felt.Felt(value)
	feltEncoding, err := asFelt.MarshalCBOR()
	require.NoError(t, err)
	require.True(t, bytes.Contains(encoded, feltEncoding))
	require.False(t, bytes.Contains(encoded, legacyCBOR(t, &asFelt)), "alias kept the legacy encoding")

	var decoded record
	require.NoError(t, cbor.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded)

	// fp.Element has no CBOR methods, so the generic encoder writes the legacy limb arrays.
	type legacyRecord struct {
		Value   fp.Element
		Pointer *fp.Element
		Nil     *fp.Element
		Values  []fp.Element
		Keys    map[fp.Element]fp.Element
	}
	smallLegacy := fp.Element(small)
	legacy, err := cbor.Marshal(legacyRecord{
		Value:   fp.Element(value),
		Pointer: &smallLegacy,
		Values:  []fp.Element{fp.Element(small), fp.Element(value)},
		Keys:    map[fp.Element]fp.Element{fp.Element(value): fp.Element(small)},
	})
	require.NoError(t, err)

	var decodedLegacy record
	require.NoError(t, cbor.Unmarshal(legacy, &decodedLegacy))
	require.Equal(t, original, decodedLegacy)
}

// Canonical map-key sorting orders felt keys by encoded length, then by value.
func TestCBORMapKeyOrder(t *testing.T) {
	keys := []felt.Felt{
		largestFelt(t),
		felt.FromUint64[felt.Felt](math.MaxUint64),
		felt.FromUint64[felt.Felt](24),
		felt.FromUint64[felt.Felt](5),
		felt.FromUint64[felt.Felt](4),
	}
	values := make(map[felt.Felt]uint8, len(keys))
	for index, key := range keys {
		values[key] = uint8(index)
	}

	encoded, err := cbor.Marshal(values)
	require.NoError(t, err)

	want := []byte{0xa0 | byte(len(keys))}
	for _, index := range []int{4, 3, 2, 1, 0} {
		key, err := keys[index].MarshalCBOR()
		require.NoError(t, err)
		want = slices.Concat(want, key, []byte{byte(index)})
	}
	require.Equal(t, want, encoded)

	var decoded map[felt.Felt]uint8
	require.NoError(t, cbor.Unmarshal(encoded, &decoded))
	require.Equal(t, values, decoded)
}

// FELT_CBOR_STRESS_N=1000000000 go test ./core/felt/ -run TestCBORStress -v -timeout 2h
func TestCBORStress(t *testing.T) {
	raw := os.Getenv("FELT_CBOR_STRESS_N")
	if raw == "" {
		t.Skip("set FELT_CBOR_STRESS_N (e.g. 1000000000) to run this sweep")
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	require.NoError(t, err)

	rng := rand.New(rand.NewSource(1))
	const logEvery = 50_000_000

	for i := range n {
		value := randomFelt[felt.Felt](rng)
		encoded, err := value.MarshalCBOR()
		require.NoError(t, err)

		var decoded felt.Felt
		require.NoError(t, decoded.UnmarshalCBOR(encoded))
		require.True(t, decoded.Equal(&value))

		if i > 0 && i%logEvery == 0 {
			t.Logf("checked %d/%d", i, n)
		}
	}
	t.Logf("checked %d random felts, 0 mismatches", n)
}

func FuzzCBORRoundTrip(f *testing.F) {
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(0), uint64(0), uint64(0), uint64(23))
	f.Add(uint64(0), uint64(0), uint64(0), uint64(24))
	f.Add(uint64(0), uint64(0), uint64(1), uint64(0))
	f.Add(uint64(1)<<59, uint64(0), uint64(0), uint64(0))
	f.Add(
		uint64(math.MaxUint64), uint64(math.MaxUint64),
		uint64(math.MaxUint64), uint64(math.MaxUint64),
	)

	f.Fuzz(func(t *testing.T, word0, word1, word2, word3 uint64) {
		var raw [32]byte
		for index, word := range []uint64{word0, word1, word2, word3} {
			for shift := range 8 {
				raw[index*8+shift] = byte(word >> (56 - 8*shift))
			}
		}
		value := felt.FromBytes[felt.Felt](raw[:])
		requireCompactRoundTrip(t, &value)

		// legacy -> compact -> compact
		var fromLegacy felt.Felt
		require.NoError(t, fromLegacy.UnmarshalCBOR(legacyCBOR(t, &value)))
		require.Equal(t, value, fromLegacy)
		requireCompactRoundTrip(t, &fromLegacy)
	})
}

// Decoding is strict: accepted compact input re-encodes to the same bytes, and anything the
// generic limb decoder accepted still decodes to the same value.
func FuzzCBORDecode(f *testing.F) {
	for _, value := range []uint64{0, 1, 23, 24, 255, 256, 65535, 65536, 1 << 32, 1 << 63} {
		valueFelt := felt.FromUint64[felt.Felt](value)
		encoded, err := valueFelt.MarshalCBOR()
		require.NoError(f, err)
		f.Add(encoded)
		f.Add(legacyCBOR(f, &valueFelt))
	}
	for _, tc := range legacyDecodeCornerCases {
		f.Add(tc.data)
	}
	f.Add(mustDecodeHex(f, "58 20"+largestFeltHex))
	f.Add(mustDecodeHex(f, "58 20"+modulusHex))
	f.Add(mustDecodeHex(f, "42 00ff"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded felt.Felt
		err := decoded.UnmarshalCBOR(data)

		if len(data) > 0 && data[0]>>5 != 0 && data[0]>>5 != 2 {
			requireLegacyDecodeEquivalent(t, data)
			return
		}
		if err != nil {
			return
		}
		reencoded, err := decoded.MarshalCBOR()
		require.NoError(t, err)
		if len(data) == 34 {
			// Montgomery input isn't checked for needing 31 bytes; that costs the conversion it saves.
			var again felt.Felt
			require.NoError(t, again.UnmarshalCBOR(reencoded))
			require.Equal(t, decoded, again)
			return
		}
		require.Equal(t, data, reencoded, "accepted a non-compact encoding")
	})
}
