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
	testBuilder := builder.New(seqAddr, bc, mockVM, utils.NewNopZapLogger(), nil)

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
	network := utils.Mainnet
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), network, log)

	// Need to store pending block create NewPendingState
	block, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)
	su, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)
	pendingGenesis := blockchain.Pending{
		Block:       block,
		StateUpdate: su,
	}
	require.NoError(t, chain.StorePending(&pendingGenesis))

	b := builder.New(new(felt.Felt).SetUint64(1), chain, vm.New(log), utils.NewNopZapLogger(), &network)

	t.Run("empty genesis config", func(t *testing.T) {
		genesisConfig := builder.GenesisConfig{}
		_, _, err := b.GenesisStateDiff(genesisConfig)
		require.NoError(t, err)
	})

	t.Run("valid non-empty genesis config", func(t *testing.T) {
		simpleStoreClassHash, err := new(felt.Felt).SetString("0x73b1d55a550a6b9073933817a40c22c4099aa5932694a85322dd5cefedbb467")
		require.NoError(t, err)

		simpleStoreAddress, err := new(felt.Felt).SetString("0xdeadbeef")
		require.NoError(t, err)

		selector, err := new(felt.Felt).SetString("0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320") // "increase_balance(amount)"
		require.NoError(t, err)

		genesisConfig := builder.GenesisConfig{
			ChainID: network.ChainIDString(),
			Classes: []string{
				"./contracts/simpleStore.json",
			},
			Contracts: map[string]builder.GenesisContractData{
				simpleStoreAddress.String(): {
					ClassHash:       *simpleStoreClassHash,
					ConstructorArgs: []felt.Felt{*new(felt.Felt).SetUint64(1)},
				},
			},
			FunctionCalls: []builder.FunctionCall{
				{
					ContractAddress:    *simpleStoreAddress,
					EntryPointSelector: *selector,
					Calldata:           []felt.Felt{*new(felt.Felt).SetUint64(2)},
				},
			},
		}

		stateDiff, newClasses, err := b.GenesisStateDiff(genesisConfig)
		require.NoError(t, err)
		balanceKey, err := new(felt.Felt).SetString("0x206f38f7e4f15e87567361213c28f235cccdaa1d7fd34c9db1dfe9489c6a091")
		require.NoError(t, err)
		balanceVal := stateDiff.StorageDiffs[*simpleStoreAddress][*balanceKey]
		require.Equal(t, balanceVal.String(), "0x3")
		require.Empty(t, stateDiff.Nonces)
		require.Equal(t, stateDiff.DeployedContracts[*simpleStoreAddress], simpleStoreClassHash)
		require.Equal(t, stateDiff.DeclaredV0Classes, []*felt.Felt{simpleStoreClassHash})
		require.Empty(t, stateDiff.DeclaredV1Classes)
		require.Empty(t, stateDiff.ReplacedClasses)
		require.NotNil(t, newClasses[*simpleStoreClassHash])
	})
}
