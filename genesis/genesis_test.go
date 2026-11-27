package genesis_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mocks"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
			require.NotNil(t, stateDiff.DeclaredV1Classes[felt.SierraClassHash(con.ClassHash)])
			require.NotNil(t, newClasses[con.ClassHash])
		}
		require.Empty(t, stateDiff.ReplacedClasses)
		require.Equal(t, len(genesisConfig.BootstrapAccounts)+3, len(stateDiff.DeployedContracts)) // num_accounts + strk token + udc + udacnt
		numFundedAccounts := 0
		v3InvokeTxnTransferAmount := "0x1111111"
		v3InvokeTxnTriggered := false
		strkAddress := felt.NewUnsafeFromString[felt.Address](
			"0x049D36570D4e46f48e99674bd3fcc84644DdD6b96F7C741B1562B82f9e004dC7",
		)
		strkTokenDiffs := stateDiff.StorageDiffs[*strkAddress]
		for _, v := range strkTokenDiffs {
			// see genesis_prefunded_accounts.json
			if v.Equal(felt.NewUnsafeFromString[felt.Felt]("0x56bc75e2d63100000")) {
				numFundedAccounts++
			}
			// see genesis_prefunded_accounts.json
			if v.Equal(felt.NewUnsafeFromString[felt.Felt](v3InvokeTxnTransferAmount)) {
				v3InvokeTxnTriggered = true
			}
		}
		require.Equal(t, len(genesisConfig.BootstrapAccounts), numFundedAccounts)
		require.True(t, v3InvokeTxnTriggered)

		// see the DEPLOY_ACCOUNT and INVOKE txns in genesis_prefund_accounts.json
		deployedClassHash := felt.NewUnsafeFromString[felt.ClassHash](
			"0x402d6191ebe3ea289789edd160f3afa6600a389f1aad0ab7709b830653c6f08",
		)
		deployedAddress := core.ContractAddress(
			new(felt.Zero[felt.Address]()),
			deployedClassHash,
			new(felt.Zero[felt.Felt]()),
			nil,
		)
		require.Equal(t, deployedClassHash, stateDiff.DeployedContracts[deployedAddress])
		invokeSender := felt.NewUnsafeFromString[felt.Address](
			"0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923",
		)
		require.Equal(t, new(felt.One[felt.Felt]()), stateDiff.Nonces[*invokeSender])
	})

	t.Run("deploy account txn gets its contract address", func(t *testing.T) {
		classHash := felt.NewUnsafeFromString[felt.ClassHash](
			"0x402d6191ebe3ea289789edd160f3afa6600a389f1aad0ab7709b830653c6f08",
		)
		salt := felt.NewUnsafeFromString[felt.Felt]("0x123")
		calldata := felt.Slice[felt.Felt]{*felt.NewFromUint64[felt.Felt](7)}
		genesisConfig := genesis.GenesisConfig{
			Txns: []rpcv10.Transaction{{
				Hash:                felt.NewFromUint64[felt.TransactionHash](1),
				Type:                rpcv10.TxnDeployAccount,
				Version:             felt.NewFromUint64[felt.Felt](1),
				Nonce:               new(felt.Zero[felt.Felt]()),
				MaxFee:              new(felt.Zero[felt.Felt]()),
				ClassHash:           classHash,
				ContractAddressSalt: salt,
				ConstructorCallData: &calldata,
				Signature:           &felt.Slice[felt.Felt]{},
			}},
		}
		want := core.ContractAddress(new(felt.Zero[felt.Address]()), classHash, salt, calldata)

		mockVM := mocks.NewMockVM(gomock.NewController(t))
		mockVM.EXPECT().BuildBlock(
			gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		).DoAndReturn(func(
			txns []core.Transaction,
			_ []core.ClassDefinition,
			_ []*felt.Felt,
			_ *vm.BlockInfo,
			_ core.StateReader,
			_ vm.BuildBlockOptions,
		) (vm.ExecutionResults, error) {
			require.Len(t, txns, 1)
			deployAccount, ok := txns[0].(*core.DeployAccountTransaction)
			require.True(t, ok)
			require.NotNil(t, deployAccount.ContractAddress)
			require.Equal(t, want, *deployAccount.ContractAddress)
			return vm.ExecutionResults{Traces: make([]vm.TransactionTrace, 1)}, nil
		})

		_, _, err := genesis.GenesisStateDiff(
			t.Context(),
			&genesisConfig,
			mockVM,
			network,
			vm.DefaultMaxSteps,
			vm.DefaultMaxGas,
			statetestutils.UseNewState(),
			nil,
		)
		require.NoError(t, err)
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
