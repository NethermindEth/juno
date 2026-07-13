package felt_test

import (
	"bytes"
	"math"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// cborArrayHeader is the CBOR header byte for an array of 4 items
const cborArrayHeader = 0x84

var decodeCornerCases = []struct {
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

func encodeBoth(value *felt.Felt) (fast, generic []byte, err error) {
	fast, err = value.MarshalCBOR()
	if err != nil {
		return nil, nil, err
	}
	generic, err = cbor.Marshal(value)
	return fast, generic, err
}

func decodeBoth(data []byte) (fast, generic felt.Felt, errFast, errGeneric error) {
	errFast = fast.UnmarshalCBOR(data)
	errGeneric = cbor.Unmarshal(data, &generic)
	return fast, generic, errFast, errGeneric
}

func requireMarshalEquivalent(t *testing.T, value *felt.Felt) []byte {
	t.Helper()

	fast, generic, err := encodeBoth(value)
	require.NoError(t, err)
	require.Equal(t, generic, fast, "fast marshal disagrees with generic marshal")
	return fast
}

func requireUnmarshalEquivalent(t *testing.T, data []byte) (decoded felt.Felt, ok bool) {
	t.Helper()

	fast, generic, errFast, errGeneric := decodeBoth(data)
	if errGeneric != nil {
		require.Error(t, errFast, "fast decoder accepted input the generic decoder rejected: % x", data)
		return felt.Felt{}, false
	}
	require.NoError(t, errFast, "fast decoder rejected input the generic decoder accepted: % x", data)
	require.True(t, fast.Equal(&generic), "decoded value mismatch for % x: fast=%s generic=%s",
		data, fast.String(), generic.String())
	return fast, true
}

func feltFromBigInt(n *big.Int) felt.Felt {
	var value felt.Felt
	value.SetBigInt(n)
	return value
}

// feltFromLimbs bypasses SetBigInt's Montgomery conversion, so a small
// argument here actually lands in a limb. SetBigInt() does not.
func feltFromLimbs(limbs ...uint64) felt.Felt {
	var l [4]uint64
	copy(l[:], limbs)
	return felt.Felt(fp.Element(l))
}

func TestCBORFastPathValues(t *testing.T) {
	type valueCase struct {
		name  string
		value felt.Felt
	}

	cases := []valueCase{
		{"zero", feltFromBigInt(big.NewInt(0))},
	}
	for _, boundary := range []struct {
		name string
		limb uint64
	}{
		{"one", 1},
		{"largest inline", 23},
		{"smallest 1-byte", 24},
		{"largest 1-byte (max uint8)", math.MaxUint8},
		{"smallest 2-byte", math.MaxUint8 + 1},
		{"largest 2-byte (max uint16)", math.MaxUint16},
		{"smallest 4-byte", math.MaxUint16 + 1},
		{"largest 4-byte (max uint32)", math.MaxUint32},
		{"smallest 8-byte", math.MaxUint32 + 1},
		{"largest 8-byte (max uint64)", math.MaxUint64},
	} {
		cases = append(cases, valueCase{"limb: " + boundary.name, feltFromLimbs(boundary.limb)})
	}
	cases = append(cases, valueCase{
		"limb: boundary values spread across all four limbs",
		feltFromLimbs(23, math.MaxUint8, math.MaxUint16, math.MaxUint64),
	})

	// Starknet's default modulus for felts.
	modulus, ok := new(big.Int).SetString(
		"3618502788666131213697322783095070105623107215331596699973092056135872020481",
		10,
	)
	require.True(t, ok)
	largestFelt := new(big.Int).Sub(modulus, big.NewInt(1))

	cases = append(cases,
		valueCase{"field modulus (reduces to zero)", feltFromBigInt(modulus)},
		valueCase{"largest valid felt (modulus - 1)", feltFromBigInt(largestFelt)},
		valueCase{"half of largest valid felt", feltFromBigInt(new(big.Int).Rsh(largestFelt, 1))},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.value

			encoded := requireMarshalEquivalent(t, &original)
			require.NotEmpty(t, encoded)
			require.Equal(t, byte(cborArrayHeader), encoded[0], "MarshalCBOR shape changed")

			decoded, ok := requireUnmarshalEquivalent(t, encoded)
			require.True(t, ok)
			require.True(t, original.Equal(&decoded), "round trip changed the value")
		})
	}
}

func TestCBORFastPathDecodeCornerCases(t *testing.T) {
	for _, tc := range decodeCornerCases {
		t.Run(tc.name, func(t *testing.T) {
			requireUnmarshalEquivalent(t, tc.data)
		})
	}
}

// FELT_CBOR_STRESS_N=1000000000 go test ./core/felt/... -run TestCBORFastPathStress -v -timeout 2h
func TestCBORFastPathStress(t *testing.T) {
	raw := os.Getenv("FELT_CBOR_STRESS_N")
	if raw == "" {
		t.Skip("set FELT_CBOR_STRESS_N (e.g. 1000000000) to run this sweep")
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	require.NoError(t, err)

	rng := rand.New(rand.NewSource(1))
	const logEvery = 50_000_000

	for i := range n {
		value := feltFromLimbs(rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64())

		fast, generic, err := encodeBoth(&value)
		require.NoError(t, err)
		require.True(t, bytes.Equal(fast, generic))

		fastDecoded, genericDecoded, errFast, errGeneric := decodeBoth(fast)
		require.NoError(t, errFast)
		require.NoError(t, errGeneric)
		require.True(t, fastDecoded.Equal(&genericDecoded))

		if i > 0 && i%logEvery == 0 {
			t.Logf("checked %d/%d", i, n)
		}
	}
	t.Logf("checked %d random felts, 0 mismatches", n)
}

func FuzzCBORFastPathUnmarshalEquivalence(f *testing.F) {
	for _, value := range []uint64{
		0, 1, 23, 24, 255, 256, 65535, 65536, 1 << 32, 1 << 63,
	} {
		var valueFelt felt.Felt
		valueFelt.SetUint64(value)

		encoded, err := valueFelt.MarshalCBOR()
		require.NoError(f, err)

		f.Add(encoded)
	}

	for _, tc := range decodeCornerCases {
		f.Add(tc.data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		requireUnmarshalEquivalent(t, data)
	})
}

func FuzzCBORFastPathMarshalEquivalence(f *testing.F) {
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(1), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(23), uint64(24), uint64(255), uint64(256))
	f.Add(uint64(65535), uint64(65536), uint64(1)<<32, uint64(1)<<63)
	f.Add(
		uint64(math.MaxUint8), uint64(math.MaxUint16),
		uint64(math.MaxUint32), uint64(math.MaxUint64),
	)
	f.Add(
		uint64(math.MaxUint64), uint64(math.MaxUint64),
		uint64(math.MaxUint64), uint64(math.MaxUint64),
	)

	f.Fuzz(func(t *testing.T, l0, l1, l2, l3 uint64) {
		value := feltFromLimbs(l0, l1, l2, l3)
		requireMarshalEquivalent(t, &value)
	})
}
