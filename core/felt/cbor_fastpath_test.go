package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/require"
)

func requireAgreesWithEngine(t *testing.T, data []byte) (decoded felt.Felt, ok bool) {
	t.Helper()

	var fast, generic felt.Felt
	errFast := fast.UnmarshalCBOR(data)
	errGeneric := encoder.Unmarshal(data, (*fp.Element)(&generic))

	if errGeneric != nil {
		require.Error(t, errFast, "took a payload the engine refuses: % x", data)
		return felt.Felt{}, false
	}
	require.NoError(t, errFast, "refused a payload the engine takes: % x", data)
	require.True(t, fast.Equal(&generic), "read % x differently: fast=%s engine=%s",
		data, fast.String(), generic.String())

	return fast, true
}

func TestFeltAccepted(t *testing.T) {
	for _, shape := range cbor.FeltAccepted {
		t.Run(shape.Name, func(t *testing.T) {
			decoded, ok := requireAgreesWithEngine(t, shape.Data)
			require.True(t, ok, "refused a payload it has to take")

			written, err := decoded.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, shape.Data, written, "wrote it back differently")
		})
	}
}

// The hook falls back to the engine, so they can never disagree.
func TestFeltRejected(t *testing.T) {
	for _, shape := range cbor.FeltRejected {
		t.Run(shape.Name, func(t *testing.T) {
			requireAgreesWithEngine(t, shape.Data)
		})
	}
}

func FuzzFelt(f *testing.F) {
	for _, shape := range append(cbor.FeltAccepted, cbor.FeltRejected...) {
		f.Add(shape.Data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		requireAgreesWithEngine(t, data)
	})
}
