package ugorji_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/ugorji"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	for _, data := range [][]byte{nil, {0xa1}, {1, 2}} {
		var value any
		require.Error(t, ugorji.Unmarshal(data, &value))
		// A failed decode must not affect the next use of the decoder pool.
		var number uint64
		require.NoError(t, ugorji.Unmarshal([]byte{7}, &number))
		require.Equal(t, uint64(7), number)
	}
}
