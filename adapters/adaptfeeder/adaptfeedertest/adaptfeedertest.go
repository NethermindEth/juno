// Package adaptfeedertest loads feeder gateway fixtures as core types for tests.
package adaptfeedertest

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/stretchr/testify/require"
)

// Block returns the block with the given number, with its signature.
func Block(t testing.TB, client feeder.Reader, number uint64) *core.Block {
	t.Helper()
	return block(t, client, strconv.FormatUint(number, 10))
}

// BlockLatest returns the latest block, with its signature.
func BlockLatest(t testing.TB, client feeder.Reader) *core.Block {
	t.Helper()
	return block(t, client, "latest")
}

func block(t testing.TB, client feeder.Reader, blockID string) *core.Block {
	t.Helper()

	response, err := client.Block(t.Context(), blockID)
	require.NoError(t, err)

	sig, err := client.Signature(t.Context(), blockID)
	require.NoError(t, err)

	block, err := sn2core.AdaptBlock(&response, sig.Signature)
	require.NoError(t, err)
	return block
}

// StateUpdate returns the state update of the block with the given number.
func StateUpdate(t testing.TB, client feeder.Reader, number uint64) *core.StateUpdate {
	t.Helper()

	response, err := client.StateUpdate(t.Context(), strconv.FormatUint(number, 10))
	require.NoError(t, err)

	stateUpdate, err := sn2core.AdaptStateUpdate(&response)
	require.NoError(t, err)
	return stateUpdate
}
