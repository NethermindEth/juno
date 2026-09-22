package rpc2core_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func feederGateway(t *testing.T, network *networks.Network) *adaptfeeder.Feeder {
	t.Helper()
	return adaptfeeder.New(feeder.NewTestClient(t, network))
}

// overWire encodes a response as the server does and decodes it as a client does.
func overWire[T any](t *testing.T, response *T) *T {
	t.Helper()
	data, err := json.Marshal(response)
	require.NoError(t, err)
	var decoded T
	require.NoError(t, json.Unmarshal(data, &decoded))
	return &decoded
}

// storeBlocks stores the first blocks of a network's fixtures in a fresh chain and returns a
// handler serving it. The classes are registered with the first block.
func storeBlocks(
	t *testing.T,
	network *networks.Network,
	blocks uint64,
	classes map[felt.Felt]core.ClassDefinition,
) (*rpcv10.Handler, *adaptfeeder.Feeder) {
	t.Helper()
	chain := blockchain.New(
		memory.New(),
		network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	gateway := feederGateway(t, network)
	for number := range blocks {
		block, err := gateway.BlockByNumber(t.Context(), number)
		require.NoError(t, err)
		stateUpdate, err := gateway.StateUpdate(t.Context(), number)
		require.NoError(t, err)
		require.NoError(t, chain.Store(block, &core.BlockCommitments{}, stateUpdate, classes))
		classes = nil
	}
	return rpcv10.New(chain, nil, nil, log.NewNopZapLogger()), gateway
}
