package builder_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateAgainstPendingState(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, utils.Integration, utils.NewNopZapLogger())
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	testBuilder := builder.New(seqAddr, bc, mockVM, utils.NewNopZapLogger())

	client := feeder.NewTestClient(t, utils.Integration)
	gw := adaptfeeder.New(client)

	su, b, err := gw.StateUpdateWithBlock(context.Background(), 0)
	require.NoError(t, err)

	require.NoError(t, bc.StorePending(&blockchain.Pending{
		Block:       b,
		StateUpdate: su,
	}))

	userTxn := mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: utils.HexToFelt(t, "0x1337"),
		},
		DeclaredClass: &core.Cairo0Class{
			Program: "best program",
		},
	}

	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, uint64(0), b.Timestamp, seqAddr,
		gomock.Any(), utils.Integration, []*felt.Felt{}, false, false,
		false, b.GasPrice, b.GasPriceSTRK, false).Return(nil, nil, nil)
	assert.NoError(t, testBuilder.ValidateAgainstPendingState(&userTxn))

	require.NoError(t, bc.Store(b, &core.BlockCommitments{}, su, nil))
	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, uint64(1), b.Timestamp+1, seqAddr,
		gomock.Any(), utils.Integration, []*felt.Felt{}, false, false,
		false, b.GasPrice, b.GasPriceSTRK, false).Return(nil, nil, errors.New("oops"))
	assert.EqualError(t, testBuilder.ValidateAgainstPendingState(&userTxn), "oops")
}

func TestGenesisStateDiff(t *testing.T) {
	log := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), utils.Goerli, log)
	b := builder.New(new(felt.Felt).SetUint64(1), chain, vm.New(log), utils.NewNopZapLogger())
	t.Run("empty genesis config", func(t *testing.T) {
		genesisConfig := builder.GenesisConfig{}
		_, err := b.GenesisStateDiff(genesisConfig)
		require.NoError(t, err)
	})
}
