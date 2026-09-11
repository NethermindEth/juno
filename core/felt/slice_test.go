package felt_test

import (
	"encoding/binary"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/v1"
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

func randomSlice[F felt.FeltLike](size int) felt.Slice[F] {
	rng := rand.New(rand.NewSource(1))
	slice := make(felt.Slice[F], size)
	for idx := range slice {
		slice[idx] = fromLimbs[F](rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64())
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
// paths and asserts they agree, either on the decoded felts or on the error.
func requireSliceDecodeCBOREquivalent[F felt.FeltLike](t *testing.T, data []byte) {
	t.Helper()

	var fast felt.Slice[F]
	errFast := fast.UnmarshalCBOR(data)

	var generic []F
	errGeneric := cbor.Unmarshal(data, &generic)

	if errGeneric != nil {
		require.Equal(
			t,
			errGeneric,
			errFast,
			"fast and generic decoders disagree on the error for % x",
			data,
		)
		return
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
			felt.Equal(&generic[i], &fast[i]),
			"element %d mismatch for % x: generic=%v fast=%v",
			i,
			data,
			generic[i],
			fast[i],
		)
	}
}

func TestSliceRoundTripCBORBoundarySizes(t *testing.T) {
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
			s := randomSlice[felt.Felt](tc.size)

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

			requireSliceDecodeCBOREquivalent[felt.Felt](t, fast)
			requireSliceDecodeCBOREquivalent[feltoid](t, fast)
		})
	}
}

// A nil slice must marshal like the generic encoder (null, not an empty array)
// and round-trip back to nil rather than an empty slice.
func TestSliceMarshalCBORNil(t *testing.T) {
	var s felt.Slice[felt.Felt] // nil

	fast, err := s.MarshalCBOR()
	require.NoError(t, err)
	generic, err := cbor.Marshal([]felt.Felt(s))
	require.NoError(t, err)
	require.Equal(t, generic, fast, "nil slice must marshal like the generic encoder")

	var back felt.Slice[feltoid]
	require.NoError(t, back.UnmarshalCBOR(fast))
	require.Nil(t, back, "nil slice must round-trip back to nil, not empty")
}

func TestSliceDecodeCBORCornerCases(t *testing.T) {
	feltBytes1 := encodeFeltCBOR(t, fromLimbs[felt.Felt](1))
	feltBytes2 := encodeFeltCBOR(t, fromLimbs[felt.Felt](2))

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
			requireSliceDecodeCBOREquivalent[felt.Felt](t, tc.data)
			requireSliceDecodeCBOREquivalent[feltoid](t, tc.data)
		})
	}
}

func TestSliceDecodeCBORRejectsOversizedHeader(t *testing.T) {
	feltBytes := encodeFeltCBOR(t, fromLimbs[felt.Felt](1))

	// cborArrayUint32 reads the element count
	// math.MaxUint32 = ~4.29 billion elements, far more than the payload holds.
	header := binary.BigEndian.AppendUint32([]byte{cborArrayUint32}, math.MaxUint32)
	data := slices.Concat(header, feltBytes)

	requireSliceDecodeCBOREquivalent[felt.Felt](t, data)
	requireSliceDecodeCBOREquivalent[feltoid](t, data)
}

// FuzzSliceDecodeEquivalence fuzzes the decode path, the one that can receive
// arbitrary bytes, to ensure it stays equivalent to the generic decoder and never panics.
func FuzzSliceDecodeCBOREquivalence(fz *testing.F) {
	for _, n := range []int{0, 1, 2, 23, 24, 255, 256} {
		encoded, err := randomSlice[felt.Felt](n).MarshalCBOR()
		require.NoError(fz, err)
		fz.Add(encoded)
	}
	for _, seed := range [][]byte{
		{cborArrayEmpty}, {cborNull}, {cborUintZero}, {cborArrayOne}, {cborArrayOne, cborUintZero}, nil,
	} {
		fz.Add(seed)
	}

	fz.Fuzz(func(t *testing.T, data []byte) {
		requireSliceDecodeCBOREquivalent[felt.Felt](t, data)
		requireSliceDecodeCBOREquivalent[feltoid](t, data)
	})
}

func TestSliceJSON(t *testing.T) {
	// One below the field modulus, so it exercises the widest hex a felt can print.
	const maxFelt = "0x800000000000011000000000000000000000000000000000000000000000000"

	cases := []struct {
		name string
		in   felt.Slice[felt.Felt]
		want string
	}{
		{"nil", nil, `null`},
		{"empty", felt.Slice[felt.Felt]{}, `[]`},
		{"one", felt.Slice[felt.Felt]{felt.UnsafeFromString[felt.Felt]("0xdeadbeef")}, `["0xdeadbeef"]`},
		{
			"many",
			felt.Slice[felt.Felt]{
				felt.Zero,
				felt.UnsafeFromString[felt.Felt]("0x1"),
				felt.UnsafeFromString[felt.Felt](maxFelt),
			},
			`["0x0","0x1","` + maxFelt + `"]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(encoded))

			// Must match the generic encoder byte for byte.
			generic, err := json.Marshal([]felt.Felt(tc.in))
			require.NoError(t, err)
			require.Equal(t, string(generic), string(encoded))

			// nil and empty have to stay distinct across the round trip.
			var roundTrip felt.Slice[felt.Felt]
			require.NoError(t, json.Unmarshal(encoded, &roundTrip))
			require.Equal(t, tc.in, roundTrip)

			requireSliceDecodeJSONEquivalent(t, encoded)
			requireSliceDecodeJSONv2Equivalent(t, encoded)
		})
	}
}

func TestSliceJSONAccepts(t *testing.T) {
	cases := map[string]string{
		"interior whitespace": `[ "0x1" , "0x2" ]`,
		"newlines":            "[\n\t\"0x1\",\n\t\"0x2\"\n]",
		"leading zeros":       `["0x0001","0x02"]`,
		"uppercase prefix":    `["0X1","0X2"]`,
		"mixed case digits":   `["0xAbCd","0xabcd"]`,
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var decoded felt.Slice[felt.Felt]
			require.NoError(t, json.Unmarshal([]byte(input), &decoded))
			require.Len(t, decoded, 2)

			requireSliceDecodeJSONEquivalent(t, []byte(input))
			requireSliceDecodeJSONv2Equivalent(t, []byte(input))
		})
	}
}

func TestSliceJSONReusesDestination(t *testing.T) {
	var slice felt.Slice[felt.Felt]
	require.NoError(t, json.Unmarshal([]byte(`["0x1","0x2","0x3","0x4","0x5"]`), &slice))
	require.Len(t, slice, 5)

	require.NoError(t, json.Unmarshal([]byte(`["0xa","0xb"]`), &slice))
	require.Len(t, slice, 2)

	encoded, err := json.Marshal(slice)
	require.NoError(t, err)
	require.Equal(t, `["0xa","0xb"]`, string(encoded))

	// And down to nothing.
	require.NoError(t, json.Unmarshal([]byte(`[]`), &slice))
	require.Empty(t, slice)
	require.NotNil(t, slice)
}

func requireSliceDecodeJSONEquivalent(t *testing.T, data []byte) {
	t.Helper()

	var fast felt.Slice[felt.Felt]
	errFast := json.Unmarshal(data, &fast)

	var generic []felt.Felt
	errGeneric := json.Unmarshal(data, &generic)

	if errGeneric != nil {
		require.Error(
			t,
			errFast,
			"fast decoder accepted input the generic decoder rejected (%v): %s",
			errGeneric,
			data,
		)

		return
	}

	require.NoError(
		t,
		errFast,
		"fast decoder rejected input the generic decoder accepted: %s",
		data,
	)

	require.Equal(t, len(generic), len(fast), "length mismatch for %s", data)
	for idx := range generic {
		require.True(
			t,
			felt.Equal(&generic[idx], &fast[idx]),
			"element %d mismatch for %s: generic=%v fast=%v",
			idx,
			data,
			generic[idx],
			fast[idx],
		)
	}
}

// requireSliceDecodeJSONv2Equivalent decodes data through the pure json/v2 API
// with both the fast and generic paths and asserts they agree on acceptance,
// on the decoded felts, and on whether the failure was a truncation.
func requireSliceDecodeJSONv2Equivalent(t *testing.T, data []byte) {
	t.Helper()

	var fast felt.Slice[felt.Felt]
	errFast := jsonv2.Unmarshal(data, &fast)

	var generic []felt.Felt
	errGeneric := jsonv2.Unmarshal(data, &generic)

	if errGeneric != nil {
		require.Error(
			t,
			errFast,
			"fast v2 decoder accepted input the generic decoder rejected (%v): %s",
			errGeneric,
			data,
		)
		// One-directional on purpose: when the generic decoder reports a
		// truncation the fast path must too, instead of fabricating a type
		// error. The reverse does not hold - on input that is both truncated
		// and has an invalid element (e.g. `[""`), the fast path validates the
		// array syntax first (EOF) while the generic one streams and fails on
		// the element first.
		if errors.Is(errGeneric, io.ErrUnexpectedEOF) {
			require.ErrorIs(
				t,
				errFast,
				io.ErrUnexpectedEOF,
				"generic decoder reported truncation for %s but the fast one masked it: %v",
				data,
				errFast,
			)
		}

		return
	}

	require.NoError(
		t,
		errFast,
		"fast v2 decoder rejected input the generic decoder accepted: %s",
		data,
	)

	require.Equal(t, len(generic), len(fast), "length mismatch for %s", data)
	for idx := range generic {
		require.True(
			t,
			felt.Equal(&generic[idx], &fast[idx]),
			"element %d mismatch for %s: generic=%v fast=%v",
			idx,
			data,
			generic[idx],
			fast[idx],
		)
	}
}

// Under the pure json/v2 API the nil-slice representation is governed by the
// FormatNilSliceAsNull option ([] by default, null under v1-compat options),
// and the fast path must stay byte-for-byte equivalent to the generic encoder.
func TestSliceJSONv2Marshal(t *testing.T) {
	options := map[string][]jsonv2.Options{
		"default":           nil,
		"nil slice as null": {jsonv2.FormatNilSliceAsNull(true)},
	}

	cases := []struct {
		name string
		in   felt.Slice[felt.Felt]
	}{
		{"nil", nil},
		{"empty", felt.Slice[felt.Felt]{}},
		{"one", felt.Slice[felt.Felt]{felt.UnsafeFromString[felt.Felt]("0xdeadbeef")}},
		{"many", randomSlice[felt.Felt](3)},
	}

	for optName, opts := range options {
		for _, tc := range cases {
			t.Run(optName+"/"+tc.name, func(t *testing.T) {
				fast, err := jsonv2.Marshal(tc.in, opts...)
				require.NoError(t, err)
				generic, err := jsonv2.Marshal([]felt.Felt(tc.in), opts...)
				require.NoError(t, err)
				require.Equal(t, string(generic), string(fast))
			})
		}
	}

	var nilSlice felt.Slice[felt.Felt]
	asArray, err := jsonv2.Marshal(nilSlice)
	require.NoError(t, err)
	require.Equal(t, `[]`, string(asArray))

	asNull, err := jsonv2.Marshal(nilSlice, jsonv2.FormatNilSliceAsNull(true))
	require.NoError(t, err)
	require.Equal(t, `null`, string(asNull))
}

func TestSliceJSONv2DecodeErrors(t *testing.T) {
	t.Run("truncated input surfaces the real decoder error", func(t *testing.T) {
		for _, input := range []string{``, `[`, `["0x1"`, `["0x1",`, `nul`, `[tru`} {
			t.Run(input, func(t *testing.T) {
				var out felt.Slice[felt.Felt]
				err := jsonv2.Unmarshal([]byte(input), &out)
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)

				requireSliceDecodeJSONv2Equivalent(t, []byte(input))
			})
		}
	})

	t.Run("truncated field value", func(t *testing.T) {
		var out struct {
			Keys felt.Slice[felt.Felt] `json:"keys"`
		}
		err := jsonv2.Unmarshal([]byte(`{"keys":`), &out)
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("wrong kind reports a type error", func(t *testing.T) {
		for _, input := range []string{`{}`, `"0x1"`, `5`, `true`} {
			t.Run(input, func(t *testing.T) {
				var out felt.Slice[felt.Felt]
				err := jsonv2.Unmarshal([]byte(input), &out)
				require.ErrorContains(t, err, "cannot unmarshal slice: unexpected json kind")

				requireSliceDecodeJSONv2Equivalent(t, []byte(input))
			})
		}
	})
}

// The wrong-kind error text can only come from UnmarshalJSONFrom (the generic
// reflection decoder never emits it), so this proves at runtime that the fast
// path is wired in under both JSON APIs, complementing the compile-time
// interface assertions in slice.go.
func TestSliceJSONFastPathWiredIn(t *testing.T) {
	unmarshalers := map[string]func([]byte, any) error{
		"v1": json.Unmarshal,
		"v2": func(data []byte, out any) error { return jsonv2.Unmarshal(data, out) },
	}

	for name, unmarshal := range unmarshalers {
		t.Run(name, func(t *testing.T) {
			var out felt.Slice[felt.Felt]
			err := unmarshal([]byte(`[1]`), &out)
			require.ErrorContains(t, err, "cannot unmarshal slice: unexpected json kind: number")
		})
	}
}

func TestSliceJSONRejects(t *testing.T) {
	for _, input := range []string{
		`{}`, `[1]`, `[null]`, `[true]`, `[[]]`,
		`["notahex"]`, `["0x"]`, `["0xzz"]`, `["0x1"}`,
		`["0xa\"b"]`, `["0x1\n"]`, `["\u00300x1"]`,
	} {
		t.Run(input, func(t *testing.T) {
			var out felt.Slice[felt.Felt]
			require.Error(t, json.Unmarshal([]byte(input), &out))

			requireSliceDecodeJSONEquivalent(t, []byte(input))
			requireSliceDecodeJSONv2Equivalent(t, []byte(input))
		})
	}
}

func FuzzSliceDecodeJSONEquivalence(fz *testing.F) {
	for _, n := range []int{0, 1, 2, 17, 256} {
		encoded, err := json.Marshal(randomSlice[felt.Felt](n))
		require.NoError(fz, err)
		fz.Add(encoded)
	}

	for _, seed := range []string{
		`null`, `[]`, `[ ]`, `["0x0"]`, `[ "0x1" , "0x2" ]`, `["0X1"]`,
		`["0xa\"b"]`, `[1]`, `[null]`, `{}`, `[`, `["0x1"`, ``,
		`{`, `nul`, `[tru`, `"0x`, `5`, `["0x1",`, `[""`,
	} {
		fz.Add([]byte(seed))
	}

	fz.Fuzz(func(t *testing.T, data []byte) {
		requireSliceDecodeJSONEquivalent(t, data)
		requireSliceDecodeJSONv2Equivalent(t, data)
	})
}
