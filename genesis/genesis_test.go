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
	
	t.Run("accounts with prefunded strk", func(t *testing.T) {
		initMintAmnt:=new(felt.Felt).SetUint64(100) // 0x64
		genesisConfig:=genesis.GenesisConfigAccountsTokens(*initMintAmnt)		
		stateDiff, newClasses, err := genesis.GenesisStateDiff(&genesisConfig, vm.New(log), network)
		require.NoError(t, err)
		require.Empty(t, stateDiff.Nonces)
		require.Equal(t, 2, len(stateDiff.DeclaredV1Classes))
		for _,con:=range genesisConfig.Contracts{
			require.NotNil(t, stateDiff.DeclaredV1Classes[con.ClassHash])
			require.NotNil(t, newClasses[con.ClassHash])
		}				
		require.Empty(t, stateDiff.ReplacedClasses)

		numFundedAccounts:=0
		strkAddress := utils.HexToFelt(t,"0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d") 
		strkTokenDiffs := stateDiff.StorageDiffs[*strkAddress]		
		for _,v:=range strkTokenDiffs{			
			if v.Equal(initMintAmnt){
				numFundedAccounts++
			}
		}		
		require.Equal(t,len(genesisConfig.BootstrapAccounts),numFundedAccounts) 
	})
}
