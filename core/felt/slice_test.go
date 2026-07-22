package felt_test

import (
	"encoding/binary"
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// CBOR initial bytes used to hand-assemble the edge-case payloads. See RFC 8949 §3.
const (
	cborUintZero    = 0x00 // unsigned integer 0 (not an array)
	cborNull        = 0xf6 // null
	cborArrayEmpty  = 0x80 // array, 0 elements
	cborArrayOne    = 0x81 // array, 1 element
	cborArrayTwo    = 0x82 // array, 2 elements
	cborArrayUint8  = 0x98 // array, element count in the following uint8
	cborArrayUint16 = 0x99 // array, element count in the following uint16
	cborArrayUint32 = 0x9a // array, element count in the following uint32
	cborArrayIndef  = 0x9f // indefinite-length array
	cborBreak       = 0xff // "break" stop code, ends an indefinite-length item
)

func randomSlice(size int) felt.Slice {
	rng := rand.New(rand.NewSource(1))
	slice := make(felt.Slice, size)
	for idx := range slice {
		slice[idx] = feltFromLimbs(rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64())
	}
	return slice
}

func encodeFeltCBOR(t *testing.T, value felt.Felt) []byte {
	t.Helper()
	encoded, err := value.MarshalCBOR()
	require.NoError(t, err)
	return encoded
}

// requireSliceDecodeEquivalent decodes data with both the fast and generic
// paths, asserts they agree, and returns whether the decode succeeded.
func requireSliceDecodeEquivalent(t *testing.T, data []byte) (ok bool) {
	t.Helper()

	var fast felt.Slice
	errFast := fast.UnmarshalCBOR(data)

	var generic []felt.Felt
	errGeneric := cbor.Unmarshal(data, &generic)

	if errGeneric != nil {
		require.Equal(
			t,
			errGeneric,
			errFast,
			"fast and generic decoders disagree on the error for % x",
			data,
		)
		return false
	}

	require.NoError(
		t,
		errFast,
		"fast decoder rejected input the generic decoder accepted: % x",
		data,
	)

	require.Equal(t, len(generic), len(fast), "length mismatch for % x", data)
	for i := range generic {
		require.True(
			t,
			generic[i].Equal(&fast[i]),
			"element %d mismatch for % x: generic=%s fast=%s",
			i,
			data,
			generic[i].String(),
			fast[i].String(),
		)
	}
	return true
}

func TestSliceRoundTripBoundarySizes(t *testing.T) {
	sizes := []struct {
		name string
		size int
	}{
		{"empty", 0},
		{"one", 1},
		{"largest inline length", 23},
		{"smallest uint8 length", 24},
		{"largest uint8 length", 255},
		{"smallest uint16 length", 256},
		{"largest uint16 length", 65535},
		{"smallest uint32 length", 65536},
	}
	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			s := randomSlice(tc.size)

			fast, err := s.MarshalCBOR()
			require.NoError(t, err)
			generic, err := cbor.Marshal([]felt.Felt(s))
			require.NoError(t, err)
			require.Equal(
				t,
				generic,
				fast,
				"fast marshal disagrees with generic array framing for len=%d",
				tc.size,
			)

			require.True(t, requireSliceDecodeEquivalent(t, fast))
		})
	}
}

// A nil slice must marshal like the generic encoder (null, not an empty array)
// and round-trip back to nil rather than an empty slice.
func TestSliceMarshalNil(t *testing.T) {
	var s felt.Slice // nil

	fast, err := s.MarshalCBOR()
	require.NoError(t, err)
	generic, err := cbor.Marshal([]felt.Felt(s))
	require.NoError(t, err)
	require.Equal(t, generic, fast, "nil slice must marshal like the generic encoder")

	var back felt.Slice
	require.NoError(t, back.UnmarshalCBOR(fast))
	require.Nil(t, back, "nil slice must round-trip back to nil, not empty")
}

func TestSliceDecodeCornerCases(t *testing.T) {
	feltBytes1 := encodeFeltCBOR(t, feltFromLimbs(1))
	feltBytes2 := encodeFeltCBOR(t, feltFromLimbs(2))

	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"nil", nil},
		{"null", []byte{cborNull}},
		{"not an array (unsigned int)", []byte{cborUintZero}},
		{"empty array", []byte{cborArrayEmpty}},
		{"one valid felt", slices.Concat([]byte{cborArrayOne}, feltBytes1)},
		{"two valid felts", slices.Concat([]byte{cborArrayTwo}, feltBytes1, feltBytes2)},
		{"array of one, no element", []byte{cborArrayOne}},
		{"array of two, second element missing", slices.Concat([]byte{cborArrayTwo}, feltBytes1)},
		{"valid array plus trailing byte", slices.Concat(
			[]byte{cborArrayOne},
			feltBytes1,
			[]byte{cborUintZero},
		)},
		{"element is not a felt-shaped array", []byte{cborArrayOne, cborUintZero}},
		{"indefinite-length array", slices.Concat(
			[]byte{cborArrayIndef},
			feltBytes1,
			[]byte{cborBreak},
		)},
		{"uint8-length header for a one-element array", slices.Concat(
			[]byte{cborArrayUint8, 0x01},
			feltBytes1,
		)},
		{"uint8-length header, missing count byte", []byte{cborArrayUint8}},
		{"uint16-length header, truncated count", []byte{cborArrayUint16, 0x00}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireSliceDecodeEquivalent(t, tc.data)
		})
	}
}

func TestSliceDecodeRejectsOversizedHeader(t *testing.T) {
	feltBytes := encodeFeltCBOR(t, feltFromLimbs(1))

	// cborArrayUint32 reads the element count
	// math.MaxUint32 = ~4.29 billion elements, far more than the payload holds.
	header := binary.BigEndian.AppendUint32([]byte{cborArrayUint32}, math.MaxUint32)
	data := slices.Concat(header, feltBytes)

	requireSliceDecodeEquivalent(t, data)
}

// FuzzSliceDecodeEquivalence fuzzes the decode path, the one that can receive
// arbitrary bytes, to ensure it stays equivalent to the generic decoder and never panics.
func FuzzSliceDecodeEquivalence(f *testing.F) {
	for _, n := range []int{0, 1, 2, 23, 24, 255, 256} {
		encoded, err := randomSlice(n).MarshalCBOR()
		require.NoError(f, err)
		f.Add(encoded)
	}
	for _, seed := range [][]byte{
		{cborArrayEmpty}, {cborNull}, {cborUintZero}, {cborArrayOne}, {cborArrayOne, cborUintZero}, nil,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		requireSliceDecodeEquivalent(t, data)
	})
}
