package feeder2core_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/feeder2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptBlock(t *testing.T) {
	tests := []struct {
		number          uint64
		protocolVersion string
		network         utils.Network
		sig             *feeder.Signature
	}{
		{
			number:  147,
			network: utils.MAINNET,
		},
		{
			number:          11817,
			protocolVersion: "0.10.1",
			network:         utils.MAINNET,
		},
		{
			number:          304740,
			protocolVersion: "0.12.1",
			network:         utils.INTEGRATION,
			sig: &feeder.Signature{
				Signature: []*felt.Felt{utils.HexToFelt(t, "0x44"), utils.HexToFelt(t, "0x37")},
			},
		},
	}

	ctx := context.Background()

	for _, test := range tests {
		t.Run(test.network.String()+" block number "+strconv.FormatUint(test.number, 10), func(t *testing.T) {
			client := feeder.NewTestClient(t, test.network)

			response, err := client.Block(ctx, strconv.FormatUint(test.number, 10))
			require.NoError(t, err)
			block, err := feeder2core.AdaptBlock(response, test.sig)
			require.NoError(t, err)

			expectedEventCount := uint64(0)
			for _, r := range response.Receipts {
				expectedEventCount += uint64(len(r.Events))
			}

			assert.NotNil(t, block.EventsBloom)
			assert.True(t, block.Hash.Equal(response.Hash))
			assert.True(t, block.ParentHash.Equal(response.ParentHash))
			assert.Equal(t, response.Number, block.Number)
			assert.True(t, block.GlobalStateRoot.Equal(response.StateRoot))
			assert.Equal(t, response.Timestamp, block.Timestamp)
			assert.Equal(t, len(response.Transactions), len(block.Transactions))
			assert.Equal(t, uint64(len(response.Transactions)), block.TransactionCount)
			if assert.Equal(t, len(response.Receipts), len(block.Receipts)) {
				for i, feederReceipt := range response.Receipts {
					assert.Equal(t, feederReceipt.ExecutionStatus == feeder.Reverted, block.Receipts[i].Reverted)
					assert.Equal(t, feederReceipt.RevertError, block.Receipts[i].RevertReason)
				}
			}
			assert.Equal(t, expectedEventCount, block.EventCount)
			assert.Equal(t, test.protocolVersion, block.ProtocolVersion)
			assert.Nil(t, block.ExtraData)

			if test.sig != nil {
				require.Len(t, block.Signatures, 1)
				assert.Equal(t, test.sig.Signature, block.Signatures[0])
			} else {
				assert.Empty(t, block.Signatures)
			}
		})
	}
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client := feeder.NewTestClient(t, utils.MAINNET)
	ctx := context.Background()

	for _, number := range numbers {
		t.Run("number "+strconv.FormatUint(number, 10), func(t *testing.T) {
			response, err := client.StateUpdate(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			feederUpdate, err := feeder2core.AdaptStateUpdate(response)
			require.NoError(t, err)

			assert.True(t, response.NewRoot.Equal(feederUpdate.NewRoot))
			assert.True(t, response.OldRoot.Equal(feederUpdate.OldRoot))
			assert.True(t, response.BlockHash.Equal(feederUpdate.BlockHash))

			assert.Equal(t, len(response.StateDiff.OldDeclaredContracts), len(feederUpdate.StateDiff.DeclaredV0Classes))
			for idx := range response.StateDiff.OldDeclaredContracts {
				resp := response.StateDiff.OldDeclaredContracts[idx]
				coreDeclaredClass := feederUpdate.StateDiff.DeclaredV0Classes[idx]
				assert.True(t, resp.Equal(coreDeclaredClass))
			}

			assert.Equal(t, len(response.StateDiff.Nonces), len(feederUpdate.StateDiff.Nonces))
			for keyStr, gw := range response.StateDiff.Nonces {
				key := utils.HexToFelt(t, keyStr)
				coreNonce := feederUpdate.StateDiff.Nonces[*key]
				assert.True(t, gw.Equal(coreNonce))
			}

			assert.Equal(t, len(response.StateDiff.DeployedContracts), len(feederUpdate.StateDiff.DeployedContracts))
			for idx := range response.StateDiff.DeployedContracts {
				gw := response.StateDiff.DeployedContracts[idx]
				coreDeployedContract := feederUpdate.StateDiff.DeployedContracts[idx]
				assert.True(t, gw.ClassHash.Equal(coreDeployedContract.ClassHash))
				assert.True(t, gw.Address.Equal(coreDeployedContract.Address))
			}

			assert.Equal(t, len(response.StateDiff.StorageDiffs), len(feederUpdate.StateDiff.StorageDiffs))
			for keyStr, diffs := range response.StateDiff.StorageDiffs {
				key := utils.HexToFelt(t, keyStr)
				coreDiffs := feederUpdate.StateDiff.StorageDiffs[*key]
				assert.Equal(t, true, len(diffs) > 0)
				assert.Equal(t, len(diffs), len(coreDiffs))
				for idx := range diffs {
					assert.Equal(t, true, diffs[idx].Key.Equal(coreDiffs[idx].Key))
					assert.Equal(t, true, diffs[idx].Value.Equal(coreDiffs[idx].Value))
				}
			}
		})
	}

	t.Run("v0.11.0 state update", func(t *testing.T) {
		integClient := feeder.NewTestClient(t, utils.INTEGRATION)

		t.Run("declared Cairo0 classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "283746")
			require.NoError(t, err)
			update, err := feeder2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.DeclaredV0Classes)
			assert.Empty(t, update.StateDiff.DeclaredV1Classes)
			assert.Empty(t, update.StateDiff.ReplacedClasses)
		})

		t.Run("declared Cairo1 classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "283364")
			require.NoError(t, err)
			update, err := feeder2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.Empty(t, update.StateDiff.DeclaredV0Classes)
			assert.NotEmpty(t, update.StateDiff.DeclaredV1Classes)
			assert.Empty(t, update.StateDiff.ReplacedClasses)
		})

		t.Run("replaced classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "283428")
			require.NoError(t, err)
			update, err := feeder2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.Empty(t, update.StateDiff.DeclaredV0Classes)
			assert.Empty(t, update.StateDiff.DeclaredV1Classes)
			assert.NotEmpty(t, update.StateDiff.ReplacedClasses)
		})
	})
}

func TestClassV0(t *testing.T) {
	classHashes := []string{
		"0x79e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
		"0x1924aa4b0bedfd884ea749c7231bafd91650725d44c91664467ffce9bf478d0",
		"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		"0x56b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
	}

	client := feeder.NewTestClient(t, utils.GOERLI)
	ctx := context.Background()

	for _, hashString := range classHashes {
		t.Run("hash "+hashString, func(t *testing.T) {
			hash := utils.HexToFelt(t, hashString)
			response, err := client.ClassDefinition(ctx, hash)
			require.NoError(t, err)
			classGeneric, err := feeder2core.AdaptCairo0Class(response.V0)
			require.NoError(t, err)
			class, ok := classGeneric.(*core.Cairo0Class)
			require.True(t, ok)

			for i, v := range response.V0.EntryPoints.External {
				assert.Equal(t, v.Selector, class.Externals[i].Selector)
				assert.Equal(t, v.Offset, class.Externals[i].Offset)
			}
			assert.Equal(t, len(response.V0.EntryPoints.External), len(class.Externals))

			for i, v := range response.V0.EntryPoints.L1Handler {
				assert.Equal(t, v.Selector, class.L1Handlers[i].Selector)
				assert.Equal(t, v.Offset, class.L1Handlers[i].Offset)
			}
			assert.Equal(t, len(response.V0.EntryPoints.L1Handler), len(class.L1Handlers))

			for i, v := range response.V0.EntryPoints.Constructor {
				assert.Equal(t, v.Selector, class.Constructors[i].Selector)
				assert.Equal(t, v.Offset, class.Constructors[i].Offset)
			}
			assert.Equal(t, len(response.V0.EntryPoints.Constructor), len(class.Constructors))

			assert.NotEmpty(t, class.Program)
		})
	}
}

func TestTransaction(t *testing.T) {
	clientGoerli := feeder.NewTestClient(t, utils.GOERLI)
	clientMainnet := feeder.NewTestClient(t, utils.MAINNET)
	ctx := context.Background()

	t.Run("invoke transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x7e3a229febf47c6edfd96582d9476dd91a58a5ba3df4553ae448a14a2f132d9")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := feeder2core.AdaptTransaction(responseTx)
		require.NoError(t, err)
		invokeTx, ok := txn.(*core.InvokeTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, invokeTx.Hash())
		assert.Equal(t, responseTx.SenderAddress, invokeTx.SenderAddress)
		assert.Equal(t, responseTx.EntryPointSelector, invokeTx.EntryPointSelector)
		assert.Equal(t, responseTx.Nonce, invokeTx.Nonce)
		assert.Equal(t, *responseTx.CallData, invokeTx.CallData)
		assert.Equal(t, *responseTx.Signature, invokeTx.Signature())
		assert.Equal(t, responseTx.MaxFee, invokeTx.MaxFee)
		assert.Equal(t, responseTx.Version, invokeTx.Version.AsFelt())
	})

	t.Run("deploy transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x15b51c2f4880b1e7492d30ada7254fc59c09adde636f37eb08cdadbd9dabebb")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := feeder2core.AdaptTransaction(responseTx)
		require.NoError(t, err)
		deployTx, ok := txn.(*core.DeployTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, deployTx.Hash())
		assert.Equal(t, responseTx.ContractAddressSalt, deployTx.ContractAddressSalt)
		assert.Equal(t, responseTx.ContractAddress, deployTx.ContractAddress)
		assert.Equal(t, responseTx.ClassHash, deployTx.ClassHash)
		assert.Equal(t, *responseTx.ConstructorCallData, deployTx.ConstructorCallData)
		assert.Equal(t, responseTx.Version, deployTx.Version.AsFelt())
	})

	t.Run("deploy account transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := feeder2core.AdaptTransaction(responseTx)
		require.NoError(t, err)
		deployAccountTx, ok := txn.(*core.DeployAccountTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, deployAccountTx.Hash())
		assert.Equal(t, responseTx.ContractAddressSalt, deployAccountTx.ContractAddressSalt)
		assert.Equal(t, responseTx.ContractAddress, deployAccountTx.ContractAddress)
		assert.Equal(t, responseTx.ClassHash, deployAccountTx.ClassHash)
		assert.Equal(t, *responseTx.ConstructorCallData, deployAccountTx.ConstructorCallData)
		assert.Equal(t, responseTx.Version, deployAccountTx.Version.AsFelt())
		assert.Equal(t, responseTx.MaxFee, deployAccountTx.MaxFee)
		assert.Equal(t, *responseTx.Signature, deployAccountTx.Signature())
		assert.Equal(t, responseTx.Nonce, deployAccountTx.Nonce)
	})

	t.Run("declare transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x6eab8252abfc9bbfd72c8d592dde4018d07ce467c5ce922519d7142fcab203f")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := feeder2core.AdaptTransaction(responseTx)
		require.NoError(t, err)
		declareTx, ok := txn.(*core.DeclareTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, declareTx.Hash())
		assert.Equal(t, responseTx.SenderAddress, declareTx.SenderAddress)
		assert.Equal(t, responseTx.Version, declareTx.Version.AsFelt())
		assert.Equal(t, responseTx.Nonce, declareTx.Nonce)
		assert.Equal(t, responseTx.MaxFee, declareTx.MaxFee)
		assert.Equal(t, *responseTx.Signature, declareTx.Signature())
		assert.Equal(t, responseTx.ClassHash, declareTx.ClassHash)
	})

	t.Run("l1handler transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := feeder2core.AdaptTransaction(responseTx)
		require.NoError(t, err)
		l1HandlerTx, ok := txn.(*core.L1HandlerTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, l1HandlerTx.Hash())
		assert.Equal(t, responseTx.ContractAddress, l1HandlerTx.ContractAddress)
		assert.Equal(t, responseTx.EntryPointSelector, l1HandlerTx.EntryPointSelector)
		assert.Equal(t, responseTx.Nonce, l1HandlerTx.Nonce)
		assert.Equal(t, *responseTx.CallData, l1HandlerTx.CallData)
		assert.Equal(t, responseTx.Version, l1HandlerTx.Version.AsFelt())
	})
}

func TestClassV1(t *testing.T) {
	client := feeder.NewTestClient(t, utils.INTEGRATION)

	classHash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")

	feederClass, err := client.ClassDefinition(context.Background(), classHash)
	require.NoError(t, err)
	compiled, err := client.CompiledClassDefinition(context.Background(), classHash)
	require.NoError(t, err)

	class, err := feeder2core.AdaptCairo1Class(feederClass.V1, compiled)
	require.NoError(t, err)

	v1Class, ok := class.(*core.Cairo1Class)
	require.True(t, ok)

	assert.Equal(t, feederClass.V1.Abi, v1Class.Abi)
	assert.Equal(t, feederClass.V1.Program, v1Class.Program)
	assert.Equal(t, feederClass.V1.Version, v1Class.SemanticVersion)
	assert.Equal(t, compiled, v1Class.Compiled)

	assert.Equal(t, len(feederClass.V1.EntryPoints.External), len(v1Class.EntryPoints.External))
	for i, v := range feederClass.V1.EntryPoints.External {
		assert.Equal(t, v.Selector, v1Class.EntryPoints.External[i].Selector)
		assert.Equal(t, v.Index, v1Class.EntryPoints.External[i].Index)
	}

	assert.Equal(t, len(feederClass.V1.EntryPoints.Constructor), len(v1Class.EntryPoints.Constructor))
	for i, v := range feederClass.V1.EntryPoints.Constructor {
		assert.Equal(t, v.Selector, v1Class.EntryPoints.Constructor[i].Selector)
		assert.Equal(t, v.Index, v1Class.EntryPoints.Constructor[i].Index)
	}

	assert.Equal(t, len(feederClass.V1.EntryPoints.L1Handler), len(v1Class.EntryPoints.L1Handler))
	for i, v := range feederClass.V1.EntryPoints.L1Handler {
		assert.Equal(t, v.Selector, v1Class.EntryPoints.L1Handler[i].Selector)
		assert.Equal(t, v.Index, v1Class.EntryPoints.L1Handler[i].Index)
	}
}
