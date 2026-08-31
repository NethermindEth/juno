package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/encoder/cbor"
	fxcbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

type limbs = [cbor.Limbs]uint64

// fxamacker wrote all initial felts on disk, so we always compare against it.
func requireAgreesWithLibrary(t *testing.T, data []byte) (decoded limbs, ok bool) {
	t.Helper()

	var fast limbs
	if !cbor.UnmarshalFelt(data, &fast) {
		return limbs{}, false
	}

	var canonical limbs
	require.NoError(t, fxcbor.Unmarshal(data, &canonical),
		"took a payload the library refuses: % x", data)
	require.Equal(t, canonical, fast)

	return fast, true
}

func TestFeltAccepted(t *testing.T) {
	for _, shape := range cbor.FeltAccepted {
		t.Run(shape.Name, func(t *testing.T) {
			decoded, ok := requireAgreesWithLibrary(t, shape.Data)
			require.True(t, ok, "refused a payload it has to take")
			require.Equal(t, shape.Data, cbor.MarshalFelt(&decoded), "wrote it back differently")
		})
	}
}

func TestFeltRejected(t *testing.T) {
	for _, shape := range cbor.FeltRejected {
		t.Run(shape.Name, func(t *testing.T) {
			_, ok := requireAgreesWithLibrary(t, shape.Data)
			require.False(t, ok, "took a payload it has to refuse")
		})
	}
}

func FuzzFelt(f *testing.F) {
	for _, shape := range append(cbor.FeltAccepted, cbor.FeltRejected...) {
		f.Add(shape.Data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		requireAgreesWithLibrary(t, data)
	})
}
