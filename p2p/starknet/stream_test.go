package starknet_test

import (
	"testing"

	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/stretchr/testify/require"
)

func TestStream(t *testing.T) {
	stream := starknet.StaticStream[int](0, 1, 2, 3)
	for i := 0; i < 4; i++ {
		next, valid := stream()
		require.True(t, valid)
		require.Equal(t, i, next)
	}
	_, valid := stream()
	require.False(t, valid)
}
