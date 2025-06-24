package genesis_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestGenesisStateDiff(t *testing.T) {
	network := &utils.Mainnet
	log := utils.NewNopZapLogger()

	t.Run("empty genesis config", func(t *testing.T) {
		genesisConfig := genesis.GenesisConfig{}
		_, _, err := genesis.GenesisStateDiff(&genesisConfig, vm.New(false, log), network, 40000000) //nolint:gomnd
		require.NoError(t, err)
	})

	t.Run("accounts with prefunded strk", func(t *testing.T) {
		// udc at 0x41a78e741e5af2fec34b695679bc6891742439f7afb8484ecd7766661ad02bf
		// udacnt at 0x535ca4e1d1be7ec4a88d51a2962cd6c5aea1be96cb2c0b60eb1721dc34f800d
		genesisConfig, err := genesis.Read("./genesis_prefund_accounts.json")
		require.NoError(t, err)
		genesisConfig.Classes = []string{"./classes/strk.json", "./classes/account.json", "./classes/universaldeployer.json", "./classes/udacnt.json"}
		stateDiff, newClasses, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), network, 40000000) //nolint:gomnd
		require.NoError(t, err)
		require.Equal(t, 2, len(stateDiff.DeclaredV1Classes))
		for _, con := range genesisConfig.Contracts {
			require.NotNil(t, stateDiff.DeclaredV1Classes[con.ClassHash])
			require.NotNil(t, newClasses[con.ClassHash])
		}
		require.Empty(t, stateDiff.ReplacedClasses)
		require.Equal(t, len(genesisConfig.BootstrapAccounts)+2, len(stateDiff.DeployedContracts)) // num_accounts + strk token + udc
		numFundedAccounts := 0
		tokensSentByInvoke3Txn := 0
		strkAddress := utils.HexToFelt(t, "0x049D36570D4e46f48e99674bd3fcc84644DdD6b96F7C741B1562B82f9e004dC7")
		strkTokenDiffs := stateDiff.StorageDiffs[*strkAddress]
		for k, v := range strkTokenDiffs {
			if v.Equal(utils.HexToFelt(t, "0x1111111")) { // see genesis_prefunded_accounts.json
				fmt.Println(" [Genesis] ", k.String(), v.String())
				numFundedAccounts++
			}
			if v.Equal(utils.HexToFelt(t, "0x54321")) { // see genesis_prefunded_accounts.json
				tokensSentByInvoke3Txn++
			}
		}
		require.Equal(t, len(genesisConfig.BootstrapAccounts)+1, numFundedAccounts) // Also fund udacnt

		// Invoke V3 txn actually executes
		require.Equal(t,
			stateDiff.Nonces[*utils.HexToFelt(t, "0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923")].String(),
			"0x1")
		require.Equal(t, 1, tokensSentByInvoke3Txn)
		panic(1)
	})
}
