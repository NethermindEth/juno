package builder_test

import (
	"context"
	"errors"
	"fmt"
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
		_, err := b.GenesisStateDiff(genesisConfig)
		require.NoError(t, err)
	})

	t.Run("valid non-empty genesis config", func(t *testing.T) {
		accountClassHash, err := new(felt.Felt).SetString("0x04d07e40e93398ed3c76981e72dd1fd22557a78ce36c0515f679e27f0bb5bc5f")
		require.NoError(t, err)

		accountAddress, err := new(felt.Felt).SetString("0xdeadbeef")
		require.NoError(t, err)

		setPubKeySelector, err := new(felt.Felt).SetString("0x00bc0eb87884ab91e330445c3584a50d7ddf4b568f02fbeb456a6242cce3f5d9")
		require.NoError(t, err)

		genesisConfig := builder.GenesisConfig{
			ChainID: network.ChainIDString(),
			Classes: []string{
				"./contracts/account.json",
			},
			Contracts: map[string]builder.GenesisContractData{
				accountAddress.String(): {
					ClassHash:       *accountClassHash,
					ConstructorArgs: []felt.Felt{*new(felt.Felt).SetUint64(222)},
				},
			},
			FunctionCalls: []builder.FunctionCall{
				{
					ContractAddress:    *accountAddress,
					EntryPointSelector: *setPubKeySelector,
					Calldata:           []felt.Felt{*new(felt.Felt).SetUint64(123)},
				},
			},
		}

		resp, err := b.GenesisStateDiff(genesisConfig)
		fmt.Println(resp, err)
		fmt.Println("resp.DeclaredV0Classes", resp.DeclaredV0Classes)
		fmt.Println("resp.DeclaredV1Classes", resp.DeclaredV1Classes)
		fmt.Println("resp.ReplacedClasses", resp.ReplacedClasses)
		fmt.Println("resp.DeployedContracts", resp.DeployedContracts)
		fmt.Println("resp.Nonces", resp.Nonces)
		fmt.Println("resp.StorageDiffs", resp.StorageDiffs)
		require.NoError(t, err)
		panic(1)
	})
}
