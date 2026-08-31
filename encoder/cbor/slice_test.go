package cbor_test

import (
	"math"
	"testing"

	"github.com/NethermindEth/juno/encoder/cbor"
	fxcbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// fxamacker wrote all initial slices on disk, so we always compare against it.
func requireSliceAgreesWithLibrary(t *testing.T, data []byte) (decoded []limbs, ok bool) {
	t.Helper()

	var fast []limbs
	if !cbor.UnmarshalFeltSlice(data, &fast) {
		return nil, false
	}

	var canonical []limbs
	require.NoError(t, fxcbor.Unmarshal(data, &canonical),
		"took a payload the library refuses: % x", data)
	require.Equal(t, canonical, fast)

	return fast, true
}

func TestFeltSliceAccepted(t *testing.T) {
	for _, accepted := range cbor.FeltSliceAccepted {
		t.Run(accepted.Name, func(t *testing.T) {
			slice := sliceOf(accepted.Size)

			canonical, err := fxcbor.Marshal(slice)
			require.NoError(t, err)
			require.Equal(t, canonical, cbor.MarshalFeltSlice(slice), "framed it differently")

			decoded, ok := requireSliceAgreesWithLibrary(t, canonical)
			require.True(t, ok, "refused the library's own bytes")
			require.Equal(t, slice, decoded, "round trip changed the slice")
		})
	}
}

func TestFeltSliceRejected(t *testing.T) {
	for _, rejected := range cbor.FeltSliceRejected {
		t.Run(rejected.Name, func(t *testing.T) {
			_, ok := requireSliceAgreesWithLibrary(t, rejected.Data)
			require.False(t, ok, "took a payload it has to refuse")
		})
	}
}

// A nil slice writes null, not an empty array.
// It is neither accepted nor rejected, it is its own case.
func TestFeltSliceNil(t *testing.T) {
	canonical, err := fxcbor.Marshal([]limbs(nil))
	require.NoError(t, err)
	require.Equal(t, canonical, cbor.MarshalFeltSlice[limbs](nil))
}

func FuzzFeltSlice(f *testing.F) {
	for _, accepted := range cbor.FeltSliceAccepted {
		f.Add(cbor.MarshalFeltSlice(sliceOf(accepted.Size)))
	}
	for _, rejected := range cbor.FeltSliceRejected {
		f.Add(rejected.Data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		requireSliceAgreesWithLibrary(t, data)
	})
}

func sliceOf(size int) []limbs {
	slice := make([]limbs, size)
	for i := range slice {
		slice[i] = limbs{uint64(i), math.MaxUint64 - uint64(i), uint64(i) << 32, math.MaxUint32}
	}
	return slice
}
