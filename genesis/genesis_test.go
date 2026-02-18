package genesis_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestGenesisStateDiff(t *testing.T) {
	network := &utils.Mainnet
	log := utils.NewNopZapLogger()

	t.Run("empty genesis config", func(t *testing.T) {
		feeTokens := utils.DefaultFeeTokenAddresses
		chainInfo := vm.ChainInfo{
			ChainID:           network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}
		genesisConfig := genesis.GenesisConfig{}
		_, _, err := genesis.GenesisStateDiff(
			t.Context(),
			&genesisConfig,
			vm.New(&chainInfo, false, log),
			network,
			vm.DefaultMaxSteps,
			vm.DefaultMaxGas,
			nil,
		)
		require.NoError(t, err)
	})

	t.Run("accounts with prefunded strk", func(t *testing.T) {
		// udc at 0x41a78e741e5af2fec34b695679bc6891742439f7afb8484ecd7766661ad02bf
		// udacnt at 0x535ca4e1d1be7ec4a88d51a2962cd6c5aea1be96cb2c0b60eb1721dc34f800d
		genesisConfig, err := genesis.Read("./genesis_prefund_accounts.json")
		require.NoError(t, err)
		genesisConfig.Classes = []string{"./classes/strk.json", "./classes/account.json", "./classes/universaldeployer.json", "./classes/udacnt.json"}

		feeTokens := utils.DefaultFeeTokenAddresses
		chainInfo := vm.ChainInfo{
			ChainID:           network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}
		stateDiff, newClasses, err := genesis.GenesisStateDiff(
			t.Context(),
			genesisConfig,
			vm.New(&chainInfo, false, log),
			network,
			vm.DefaultMaxSteps,
			vm.DefaultMaxGas,
			compiler.NewUnsafe(),
		)
		require.NoError(t, err)
		require.Equal(t, 2, len(stateDiff.DeclaredV1Classes))
		for _, con := range genesisConfig.Contracts {
			require.NotNil(t, stateDiff.DeclaredV1Classes[con.ClassHash])
			require.NotNil(t, newClasses[con.ClassHash])
		}
		require.Empty(t, stateDiff.ReplacedClasses)
		require.Equal(t, len(genesisConfig.BootstrapAccounts)+3, len(stateDiff.DeployedContracts)) // num_accounts + strk token + udc + udacnt
		numFundedAccounts := 0
		v3InvokeTxnTransferAmount := "0x1111111"
		v3InvokeTxnTriggered := false
		strkAddress := felt.NewUnsafeFromString[felt.Felt]("0x049D36570D4e46f48e99674bd3fcc84644DdD6b96F7C741B1562B82f9e004dC7")
		strkTokenDiffs := stateDiff.StorageDiffs[*strkAddress]
		for _, v := range strkTokenDiffs {
			if v.Equal(felt.NewUnsafeFromString[felt.Felt]("0x56bc75e2d63100000")) { // see genesis_prefunded_accounts.json
				numFundedAccounts++
			}
			if v.Equal(felt.NewUnsafeFromString[felt.Felt](v3InvokeTxnTransferAmount)) { // see genesis_prefunded_accounts.json
				v3InvokeTxnTriggered = true
			}
		}
		require.Equal(t, len(genesisConfig.BootstrapAccounts), numFundedAccounts)
		require.True(t, v3InvokeTxnTriggered)
	})
}
