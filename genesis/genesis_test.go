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
		// Class hashes
		simpleStoreClassHash, err := new(felt.Felt).SetString("0x73b1d55a550a6b9073933817a40c22c4099aa5932694a85322dd5cefedbb467")
		require.NoError(t, err)

		simpleAccountClassHash, err := new(felt.Felt).SetString("0x04c6d6cf894f8bc96bb9c525e6853e5483177841f7388f74a46cfda6f028c755") // account (udc deploys an instance of this class)
		require.NoError(t, err)
	
		strkClassHash, err := new(felt.Felt).SetString("0x04ad3c1dc8413453db314497945b6903e1c766495a1e60492d44da9c2a986e4b")
		require.NoError(t, err)

		udcClassHash, err := new(felt.Felt).SetString("0x07b3e05f48f0c69e4a65ce5e076a66271a527aff2c34ce1083ec6e1526997a69")
		require.NoError(t, err)

		// Contract addresses
		simpleStoreAddress, err := new(felt.Felt).SetString("0xdeadbeef") // This simple contract helps with testing (no assertions/validations etc)
		require.NoError(t, err)

		strkAddress, err := new(felt.Felt).SetString("0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d") // strk token
		require.NoError(t, err)

		accounts:=genesis.Accounts()
	
		udcAddress, err := new(felt.Felt).SetString("0xdeadbeef222") // UDC contract - allows deploying contracts using invoke txns
		require.NoError(t, err)

		selector, err := new(felt.Felt).SetString("0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320") // "increase_balance(amount)"
		require.NoError(t, err)

		whyIsThisNeeded := new(felt.Felt).SetUint64(6) // Buffer for self parameter??
		permissionedMinter := utils.HexToFelt(t,"0x123456")
		initialMintAmnt:=utils.HexToFelt(t,"0x000111")
		// Pretty sure this is the token contract
		// https://github.com/starknet-io/starkgate-contracts/blob/cairo-1/src/openzeppelin/token/erc20_v070/erc20.cairo#L110
		strkConstrcutorArgs := []felt.Felt{
			*utils.HexToFelt(t,"0x537461726b6e657420546f6b656e"), 	// 1 name, felt
			*utils.HexToFelt(t,"0x5354524b"), 						// 2 symbol, felt
			*utils.HexToFelt(t,"0x12"), 							// 3 decimals, u8
			*utils.HexToFelt(t,"0x123456789"),						// 4 initial_supply, u256
			*permissionedMinter, 									// 5 recipient, ContractAddress
			*permissionedMinter, 							 		// 6 permitted_minter, ContractAddress
			*permissionedMinter,	 								// 7 provisional_governance_admin, ContractAddress
			*utils.HexToFelt(t,"0x1"), 								// 8 upgrade_delay, u128
			*whyIsThisNeeded, 										// Todo: ? ref self: ContractState ?
		} 
	
		genesisConfig := genesis.GenesisConfig{
			Classes: []string{
				"./testdata/simpleStore.json",
				"./testdata/simpleAccount.json",
				"./testdata/universalDeployer.json",	
				"./testdata/strk.json",			
			},
			// To deploy an account, we can call the constructor directly
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
				// deploy strk
				*strkAddress: {
					ClassHash: *strkClassHash,
					ConstructorArgs: strkConstrcutorArgs,
				},
			},
			// When the account is deployed, we can call any function (eg increase balance)
			FunctionCalls: []genesis.FunctionCall{
				{
					// increase balance (just a variable, not a token balance)
					ContractAddress:    *simpleStoreAddress,
					EntryPointSelector: *selector,
					Calldata:           []felt.Felt{*new(felt.Felt).SetUint64(2)},				
				},
			},
		}

		// deploy accounts
		for _, acnt:=range accounts{			
			genesisConfig.Contracts[acnt.Address]=genesis.GenesisContractData{
					ClassHash: *simpleAccountClassHash,
					ConstructorArgs: []felt.Felt{acnt.PubKey},
				}
		}

		// fund accounts with strk token
		for _, acnt:=range accounts{			
			genesisConfig.FunctionCalls =append(genesisConfig.FunctionCalls,
				genesis.FunctionCall{
					ContractAddress:    *strkAddress,
					EntryPointSelector: *utils.HexToFelt(t,"0x0083afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"), // transfer
					Calldata:           []felt.Felt{acnt.Address, *initialMintAmnt, *whyIsThisNeeded},       // todo
					CallerAddress: *permissionedMinter,
			})			
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
		require.Equal(t, stateDiff.DeclaredV0Classes[0].String(), simpleStoreClassHash.String()) // todo: flaky, may not be ordered
		require.Equal(t, stateDiff.DeclaredV0Classes[1].String(), udcClassHash.String())
		require.Equal(t, 2, len(stateDiff.DeclaredV1Classes))
		require.NotNil(t, stateDiff.DeclaredV1Classes[*simpleAccountClassHash])
		require.NotNil(t, stateDiff.DeclaredV1Classes[*strkClassHash])
		require.Empty(t, stateDiff.ReplacedClasses)
		require.NotNil(t, newClasses[*simpleStoreClassHash])
		require.NotNil(t, newClasses[*simpleAccountClassHash])
		require.NotNil(t, newClasses[*strkClassHash])

		numFundedAccounts:=0
		strkTokenDiffs := stateDiff.StorageDiffs[*strkAddress]		
		for _,v:=range strkTokenDiffs{			
			if v.Equal(initialMintAmnt){
				numFundedAccounts++
			}
		}		
		require.Equal(t,len(accounts),numFundedAccounts)
	})
}
