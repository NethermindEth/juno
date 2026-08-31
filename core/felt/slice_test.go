package felt_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cbor"
	"github.com/stretchr/testify/require"
)

func randomSlice[F felt.FeltLike](size int) felt.Slice[F] {
	rng := rand.New(rand.NewSource(1))
	slice := make(felt.Slice[F], size)
	for idx := range slice {
		slice[idx] = fromLimbs[F](rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64())
	}
	return slice
}

func requireSliceAgreesWithEngine[F felt.FeltLike](
	t *testing.T,
	data []byte,
) (felt.Slice[F], bool) {
	t.Helper()

	var fast felt.Slice[F]
	var generic []F
	errFast := fast.UnmarshalCBOR(data)
	errGeneric := encoder.Unmarshal(data, &generic)

	if errGeneric != nil {
		require.Error(t, errFast, "took a payload the engine refuses: % x", data)
		return nil, false
	}
	require.NoError(t, errFast, "refused a payload the engine takes: % x", data)
	require.Equal(t, generic, []F(fast), "read % x differently", data)

	return fast, true
}

func TestSliceAccepted(t *testing.T) {
	for _, accepted := range cbor.FeltSliceAccepted {
		t.Run(accepted.Name, func(t *testing.T) {
			slice := randomSlice[felt.Felt](accepted.Size)

			fast, err := slice.MarshalCBOR()
			require.NoError(t, err)
			generic, err := encoder.Marshal([]felt.Felt(slice))
			require.NoError(t, err)
			require.Equal(t, generic, fast, "framed it differently")

			decoded, ok := requireSliceAgreesWithEngine[felt.Felt](t, fast)
			require.True(t, ok, "refused its own output")
			require.Equal(t, slice, decoded, "round trip changed the slice")

			_, ok = requireSliceAgreesWithEngine[feltoid](t, fast)
			require.True(t, ok)
		})
	}
}

// The hook falls back to the engine, so they can never disagree.
func TestSliceRejected(t *testing.T) {
	for _, shape := range cbor.FeltSliceRejected {
		t.Run(shape.Name, func(t *testing.T) {
			requireSliceAgreesWithEngine[felt.Felt](t, shape.Data)
			requireSliceAgreesWithEngine[feltoid](t, shape.Data)
		})
	}
}

// A nil slice writes null, not an empty array, and reads back as nil.
func TestSliceNil(t *testing.T) {
	var slice felt.Slice[felt.Felt]

	fast, err := slice.MarshalCBOR()
	require.NoError(t, err)
	generic, err := encoder.Marshal([]felt.Felt(slice))
	require.NoError(t, err)
	require.Equal(t, generic, fast)

	var back felt.Slice[feltoid]
	require.NoError(t, back.UnmarshalCBOR(fast))
	require.Nil(t, back, "nil has to round trip back to nil, not empty")
}

func FuzzSliceCBOR(fz *testing.F) {
	for _, accepted := range cbor.FeltSliceAccepted {
		encoded, err := randomSlice[felt.Felt](accepted.Size).MarshalCBOR()
		require.NoError(fz, err)
		fz.Add(encoded)
	}
	for _, rejected := range cbor.FeltSliceRejected {
		fz.Add(rejected.Data)
	}

	fz.Fuzz(func(t *testing.T, data []byte) {
		requireSliceAgreesWithEngine[felt.Felt](t, data)
		requireSliceAgreesWithEngine[feltoid](t, data)
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
	} {
		fz.Add([]byte(seed))
	}

	fz.Fuzz(func(t *testing.T, data []byte) {
		requireSliceDecodeJSONEquivalent(t, data)
	})
}
