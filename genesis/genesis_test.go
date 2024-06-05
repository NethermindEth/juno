package genesis_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/genesis"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestGenesisStateDiff(t *testing.T) {
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()
	chain := blockchain.New(pebble.NewMemTest(t), network)

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

	t.Run("empty genesis config", func(t *testing.T) {
		genesisConfig := genesis.GenesisConfig{}
		_, _, err := genesis.GenesisStateDiff(&genesisConfig, vm.New(log), network)
		require.NoError(t, err)
	})

	t.Run("valid non-empty genesis config", func(t *testing.T) {

		strkAddress, err := new(felt.Felt).SetString("0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")
		require.NoError(t, err)
		strkClassHash, err := new(felt.Felt).SetString("0xbbea34049b08de10210a8a62808314f289ed56ffdb0d3d2212d9b4806470bb")
		require.NoError(t, err)

		newAccountAddress := utils.HexToFelt(t, "0xbabe") // Todo: create account using some class and use that

		strkConstructorArgs := []felt.Felt{
			*newAccountAddress, // owner
		}

		simpleStoreClassHash, err := new(felt.Felt).SetString("0x73b1d55a550a6b9073933817a40c22c4099aa5932694a85322dd5cefedbb467")
		require.NoError(t, err)

		simpleAccountClassHash, err := new(felt.Felt).SetString("0x04c6d6cf894f8bc96bb9c525e6853e5483177841f7388f74a46cfda6f028c755")
		require.NoError(t, err)

		simpleStoreAddress, err := new(felt.Felt).SetString("0xdeadbeef")
		require.NoError(t, err)

		selector, err := new(felt.Felt).SetString("0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320") // "increase_balance(amount)"
		require.NoError(t, err)

		permissionedMintSelector, err := new(felt.Felt).SetString("0x2f0b3c5710379609eb5495f1ecd348cb28167711b73609fe565a72734550354") // "mint"
		require.NoError(t, err)

		udcClassHash, err := new(felt.Felt).SetString("0x07b3e05f48f0c69e4a65ce5e076a66271a527aff2c34ce1083ec6e1526997a69")
		require.NoError(t, err)

		udcAddress, err := new(felt.Felt).SetString("0xdeadbeef222")
		require.NoError(t, err)

		genesisConfig := genesis.GenesisConfig{
			Classes: []string{
				"./testdata/ERC20_Mintable_OZ_0.8.1.json", // We use this as the fee token
				"./testdata/simpleStore.json",
				"./testdata/simpleAccount.json",
				"./testdata/universalDeployer.json",
			},
			Contracts: map[felt.Felt]genesis.GenesisContractData{ // Call the constructor
				*strkAddress: {
					ClassHash:       *strkClassHash,
					ConstructorArgs: strkConstructorArgs,
				},
				*simpleStoreAddress: {
					ClassHash:       *simpleStoreClassHash,
					ConstructorArgs: []felt.Felt{*new(felt.Felt).SetUint64(1)},
				},
				*udcAddress: {
					ClassHash: *udcClassHash,
				},
			},
			FunctionCalls: []genesis.FunctionCall{ // Todo: mint some tokens!!
				{
					ContractAddress:    *simpleStoreAddress,
					EntryPointSelector: *selector,
					Calldata:           []felt.Felt{*new(felt.Felt).SetUint64(2)},
				},
				{
					ContractAddress:    *strkAddress,
					EntryPointSelector: *permissionedMintSelector,
					Calldata:           []felt.Felt{*newAccountAddress, *newAccountAddress, *new(felt.Felt).SetUint64(2000000)}, // todo: should only be two... (first is self??)
				},
			},
		}

		stateDiff, newClasses, err := genesis.GenesisStateDiff(&genesisConfig, vm.New(log), network)
		require.NoError(t, err)
		balanceKey, err := new(felt.Felt).SetString("0x206f38f7e4f15e87567361213c28f235cccdaa1d7fd34c9db1dfe9489c6a091")
		require.NoError(t, err)
		balanceVal := stateDiff.StorageDiffs[*simpleStoreAddress][*balanceKey]
		require.Equal(t, balanceVal.String(), "0x3")
		require.Empty(t, stateDiff.Nonces)
		require.Equal(t, stateDiff.DeployedContracts[*simpleStoreAddress], simpleStoreClassHash)
		require.Equal(t, stateDiff.DeployedContracts[*udcAddress], udcClassHash)
		require.Equal(t, stateDiff.DeclaredV0Classes[0].String(), simpleStoreClassHash.String())
		require.Equal(t, stateDiff.DeclaredV0Classes[1].String(), udcClassHash.String())
		require.Equal(t, 2, len(stateDiff.DeclaredV1Classes))
		require.NotNil(t, stateDiff.DeclaredV1Classes[*simpleAccountClassHash])
		require.NotNil(t, stateDiff.DeclaredV1Classes[*strkClassHash])
		require.Empty(t, stateDiff.ReplacedClasses)
		require.NotNil(t, newClasses[*strkClassHash])
		require.NotNil(t, newClasses[*simpleStoreClassHash])
		require.NotNil(t, newClasses[*simpleAccountClassHash])

		// Todo : we minted coins

	})
}
