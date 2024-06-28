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

	t.Run("valid   non-empty genesis config", func(t *testing.T) {
		simpleStoreClassHash, err := new(felt.Felt).SetString("0x73b1d55a550a6b9073933817a40c22c4099aa5932694a85322dd5cefedbb467")
		require.NoError(t, err)

		simpleAccountClassHash, err := new(felt.Felt).SetString("0x04c6d6cf894f8bc96bb9c525e6853e5483177841f7388f74a46cfda6f028c755") // account (udc deploys an instance of this class)
		require.NoError(t, err)
	
		// deployContractSelector, err := new(felt.Felt).SetString("0x01987cbd17808b9a23693d4de7e246a443cfe37e6e7fbaeabd7d7e6532b07c3d") // "deployContract(classHash, salt, unique, calldata)"
		// require.NoError(t, err)

		udcClassHash, err := new(felt.Felt).SetString("0x07b3e05f48f0c69e4a65ce5e076a66271a527aff2c34ce1083ec6e1526997a69")
		require.NoError(t, err)


		simpleStoreAddress, err := new(felt.Felt).SetString("0xdeadbeef") // token
		require.NoError(t, err)

		simpleAccountAddress, err := new(felt.Felt).SetString("0xdeadbeef123") // account
		require.NoError(t, err)
	

		udcAddress, err := new(felt.Felt).SetString("0xdeadbeef222")
		require.NoError(t, err)

		selector, err := new(felt.Felt).SetString("0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320") // "increase_balance(amount)"
		require.NoError(t, err)

		simpleAccountPubKey, err := new(felt.Felt).SetString("0xdeadbeef123") // account pubkey
		require.NoError(t, err)

		genesisConfig := genesis.GenesisConfig{
			Classes: []string{
				"./testdata/simpleStore.json",
				"./testdata/simpleAccount.json",
				"./testdata/universalDeployer.json",		
			},
			// To deploy an account, we can call the constructor directly (don't need udc)
			Contracts: map[felt.Felt]genesis.GenesisContractData{
				// deploy token
				*simpleStoreAddress: {
					ClassHash:       *simpleStoreClassHash,
					ConstructorArgs: []felt.Felt{*new(felt.Felt).SetUint64(1)},
				},
				// deploy UDC 
				*udcAddress: {
					ClassHash: *udcClassHash,
				},
				// deploy account
				*simpleAccountAddress: {
					ClassHash: *simpleAccountClassHash,
					ConstructorArgs: []felt.Felt{*simpleAccountPubKey},
				},
			},
			// When the account is deployed, we can call any function (eg increase balance)
			FunctionCalls: []genesis.FunctionCall{
				{
					// increase balance (of caller??)
					ContractAddress:    *simpleStoreAddress,
					EntryPointSelector: *selector,
					Calldata:           []felt.Felt{*new(felt.Felt).SetUint64(2)},
				},
			},
		}

		// Todo: update to test the account has been deployed and funded.
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
		require.Equal(t, 1, len(stateDiff.DeclaredV1Classes))
		require.NotNil(t, stateDiff.DeclaredV1Classes[*simpleAccountClassHash])
		require.Empty(t, stateDiff.ReplacedClasses)
		require.NotNil(t, newClasses[*simpleStoreClassHash])
		require.NotNil(t, newClasses[*simpleAccountClassHash])
	})
}
