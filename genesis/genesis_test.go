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
		strkClassHash, err := new(felt.Felt).SetString("0x04ad3c1dc8413453db314497945b6903e1c766495a1e60492d44da9c2a986e4b")
		require.NoError(t, err)

		newAccountAddress := utils.HexToFelt(t, "0xbabe") // Todo: create account using some class and use that

		strkConstructorArgs := []felt.Felt{
			*utils.HexToFelt(t, "0x537461726b6e657420546f6b656e"), // name
			*utils.HexToFelt(t, "0x5354524b"),                     // symbol
			*utils.HexToFelt(t, "0x12"),                           // decimals
			*utils.HexToFelt(t, "0x08d669aba6e147502d0000"),       // initial_supply
			*newAccountAddress,                                    // recipient 						// Todo
			*newAccountAddress,                                    // permitted_minter 					// Todo
			*newAccountAddress,                                    // provisional_governance_admin 		// Todo
			*new(felt.Felt).SetUint64(1),                          // upgrade_delay
			*new(felt.Felt).SetUint64(1),                          // ???
		}

		simpleStoreClassHash, err := new(felt.Felt).SetString("0x73b1d55a550a6b9073933817a40c22c4099aa5932694a85322dd5cefedbb467")
		require.NoError(t, err)

		simpleAccountClassHash, err := new(felt.Felt).SetString("0x04c6d6cf894f8bc96bb9c525e6853e5483177841f7388f74a46cfda6f028c755")
		require.NoError(t, err)

		simpleStoreAddress, err := new(felt.Felt).SetString("0xdeadbeef")
		require.NoError(t, err)

		selector, err := new(felt.Felt).SetString("0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320") // "increase_balance(amount)"
		require.NoError(t, err)

		permissionedMintSelector, err := new(felt.Felt).SetString("0x01c67057e2995950900dbf33db0f5fc9904f5a18aae4a3768f721c43efe5d288") // "permissioned_mint(account,amount)"
		require.NoError(t, err)

		udcClassHash, err := new(felt.Felt).SetString("0x07b3e05f48f0c69e4a65ce5e076a66271a527aff2c34ce1083ec6e1526997a69")
		require.NoError(t, err)

		udcAddress, err := new(felt.Felt).SetString("0xdeadbeef222")
		require.NoError(t, err)

		genesisConfig := genesis.GenesisConfig{
			Classes: []string{
				"./testdata/strk.json",
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
					Calldata:           []felt.Felt{*newAccountAddress, *new(felt.Felt).SetUint64(2000000), *new(felt.Felt).SetUint64(2000000)},
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
