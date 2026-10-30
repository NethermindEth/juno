package genesis_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/genesis"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestGenesisStateDiff(t *testing.T) {
	network := &networks.Mainnet
	logger := log.NewNopZapLogger()

	t.Run("empty genesis config", func(t *testing.T) {
		feeTokens := networks.DefaultFeeTokenAddresses
		chainInfo := vm.ChainInfo{
			ChainID:           network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}
		genesisConfig := genesis.GenesisConfig{}
		_, _, err := genesis.GenesisStateDiff(
			t.Context(),
			&genesisConfig,
			vm.New(&chainInfo, false, logger),
			network,
			vm.DefaultMaxSteps,
			vm.DefaultMaxGas,
			statetestutils.UseNewState(),
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

		feeTokens := networks.DefaultFeeTokenAddresses
		chainInfo := vm.ChainInfo{
			ChainID:           network.L2ChainID,
			FeeTokenAddresses: feeTokens,
		}
		stateDiff, newClasses, err := genesis.GenesisStateDiff(
			t.Context(),
			genesisConfig,
			vm.New(&chainInfo, false, logger),
			network,
			vm.DefaultMaxSteps,
			vm.DefaultMaxGas,
			statetestutils.UseNewState(),
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

		// see the DEPLOY_ACCOUNT and INVOKE txns in genesis_prefund_accounts.json
		deployedClassHash := felt.NewUnsafeFromString[felt.Felt](
			"0x402d6191ebe3ea289789edd160f3afa6600a389f1aad0ab7709b830653c6f08",
		)
		deployedAddress := core.ContractAddress(
			felt.ZeroPtr[felt.Felt](),
			deployedClassHash,
			felt.ZeroPtr[felt.Felt](),
			nil,
		)
		require.Equal(t, deployedClassHash, stateDiff.DeployedContracts[deployedAddress])
		invokeSender := felt.NewUnsafeFromString[felt.Felt](
			"0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923",
		)
		require.Equal(t, felt.OnePtr[felt.Felt](), stateDiff.Nonces[*invokeSender])
	})
}

func TestReadTransactions(t *testing.T) {
	genesisConfig, err := genesis.Read("./genesis_prefund_accounts.json")
	require.NoError(t, err)
	require.Len(t, genesisConfig.Txns, 2)

	invoke := genesisConfig.Txns[1]
	require.Equal(t, rpcv10.TxnInvoke, invoke.Type)
	bound := felt.NewUnsafeFromString[felt.Felt]("0x128")
	l2GasAmount := felt.NewUnsafeFromString[felt.Felt]("0x6b6c1")
	require.Equal(t, &rpcv10.ResourceBoundsMap{
		L1Gas:     rpcv10.ResourceBounds{MaxAmount: bound, MaxPricePerUnit: bound},
		L2Gas:     rpcv10.ResourceBounds{MaxAmount: l2GasAmount, MaxPricePerUnit: bound},
		L1DataGas: rpcv10.ResourceBounds{MaxAmount: bound, MaxPricePerUnit: bound},
	}, invoke.ResourceBounds)
}
