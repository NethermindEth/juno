package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func TestFxamackerGoldenBytes(t *testing.T) {
	cases := goldenCases()
	golden := goldenBytes(t, goldenFile())
	t.Run("encoding", func(t *testing.T) {
		assertEncodesToGolden(t, fxamacker.Marshal, golden, cases)
	})
	require.Equal(t, len(cases), len(golden), "a case lost its vector")

	// Every build decodes both felt forms
	for _, stored := range []struct{ codec, file string }{
		{"compact", compactGoldenFile},
		{"legacy", legacyGoldenFile},
	} {
		golden := goldenBytes(t, stored.file)
		t.Run("decoding "+stored.codec, func(t *testing.T) {
			assertDecodesFromGolden(t, fxamacker.Unmarshal, golden, cases)
		})
		require.Equal(t, len(cases), len(golden), "a case lost its %s vector", stored.codec)
	}
}

func TestFxamackerRawMessage(t *testing.T) {
	rawMessageContract[fxamacker.RawMessage](t, fxamacker.Marshal, fxamacker.Unmarshal)
}
