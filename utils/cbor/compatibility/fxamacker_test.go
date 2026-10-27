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
	t.Run("decoding", func(t *testing.T) {
		assertDecodesFromGolden(t, fxamacker.Unmarshal, golden, cases)
	})
	require.Equal(t, len(cases), len(golden), "a case lost its vector")

	if !compactFeltWrites {
		return
	}
	legacy := goldenBytes(t, legacyGoldenFile)
	t.Run("decoding legacy", func(t *testing.T) {
		assertDecodesFromGolden(t, fxamacker.Unmarshal, legacy, cases)
	})
	require.Equal(t, len(cases), len(legacy), "a case lost its legacy vector")
}

func TestFxamackerRawMessage(t *testing.T) {
	rawMessageContract[fxamacker.RawMessage](t, fxamacker.Marshal, fxamacker.Unmarshal)
}
