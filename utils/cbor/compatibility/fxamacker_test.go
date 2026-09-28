package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func TestFxamackerGoldenBytes(t *testing.T) {
	cases := goldenCases()
	t.Run("encoding", func(t *testing.T) {
		goldenEncoding(t, fxamacker.Marshal, cases)
	})
	t.Run("decoding", func(t *testing.T) {
		goldenDecoding(t, fxamacker.Unmarshal, cases)
	})
	require.Equal(t, len(cases), len(goldenBytes(t)), "a case lost its vector")
}

func TestFxamackerRawMessage(t *testing.T) {
	rawMessageContract[fxamacker.RawMessage](t, fxamacker.Marshal, fxamacker.Unmarshal)
}
