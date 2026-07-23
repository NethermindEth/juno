package builder_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// InitPreconfirmedBlock leaves EventsBloom nil; Finish must populate it.
// GetBlockHeaderEventsBloomByNumber errors on a nil bloom, so a regression
// here would break the running event filter on the sequencer path.
func TestFinishSetsEventsBloom(t *testing.T) {
	bc := blockchain.New(
		memory.New(),
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	emptyStateDiff := core.EmptyStateDiff()
	require.NoError(t, bc.StoreGenesis(&emptyStateDiff, nil))

	mockVM := mocks.NewMockVM(gomock.NewController(t))
	executor := builder.NewExecutor(bc, mockVM, log.NewNopZapLogger(), false, false)
	testBuilder := builder.New(bc, executor)

	params := builder.BuildParams{
		Builder:           felt.Zero,
		Timestamp:         uint64(time.Now().Unix()),
		L2GasPriceFRI:     felt.One,
		L1GasPriceWEI:     felt.One,
		L1DataGasPriceWEI: felt.One,
		EthToStrkRate:     felt.One,
		L1DAMode:          core.Blob,
	}

	state, err := testBuilder.InitPreconfirmedBlock(&params)
	require.NoError(t, err)
	require.Nil(t, state.PreConfirmed.Block.EventsBloom)

	result, err := testBuilder.Finish(state)
	require.NoError(t, err)

	block := result.PreConfirmed.Block
	require.NotNil(t, block.EventsBloom)
	require.Equal(t, core.EventsBloom(block.Receipts), block.EventsBloom)
}
