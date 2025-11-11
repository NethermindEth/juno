package sn2core_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptBlock(t *testing.T) {
	tests := []struct {
		number          uint64
		protocolVersion string
		network         utils.Network
		sig             *starknet.Signature
		l1GasPriceWEI   *felt.Felt
		l1GasPriceSTRK  *felt.Felt
		l1DAGasPriceWEI *felt.Felt
		l1DAGasPriceFRI *felt.Felt
		l2GasPriceWEI   *felt.Felt
		l2GasPriceFRI   *felt.Felt
	}{
		{
			number:        147,
			network:       utils.Mainnet,
			l1GasPriceWEI: &felt.Zero,
		},
		{
			number:          11817,
			protocolVersion: "0.10.1",
			network:         utils.Mainnet,
			l1GasPriceWEI:   felt.NewUnsafeFromString[felt.Felt]("0x27ad16775"),
		},
		{
			number:          304740,
			protocolVersion: "0.12.1",
			network:         utils.Integration,
			sig: &starknet.Signature{
				Signature: []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x44"), felt.NewUnsafeFromString[felt.Felt]("0x37")},
			},
			l1GasPriceWEI: felt.NewUnsafeFromString[felt.Felt]("0x3bb2acbc"),
		},
		{
			number:          319132,
			network:         utils.Integration,
			protocolVersion: "0.13.0",
			sig: &starknet.Signature{
				Signature: []*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16"),
					felt.NewUnsafeFromString[felt.Felt]("0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"),
				},
			},
			l1GasPriceWEI:  felt.NewUnsafeFromString[felt.Felt]("0x3b9aca08"),
			l1GasPriceSTRK: felt.NewUnsafeFromString[felt.Felt]("0x2540be400"),
		},
		{
			number:          330363,
			network:         utils.Integration,
			protocolVersion: "0.13.1",
			sig: &starknet.Signature{
				Signature: []*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0x7685fbcd4bacae7554ad17c6962221143911d894d7b8794d29234623f392562"),
					felt.NewUnsafeFromString[felt.Felt]("0x343e605de3957e664746ba8ef82f2b0f9d53cda3d75dcb078290b8edd010165"),
				},
			},
			l1GasPriceWEI:   felt.NewUnsafeFromString[felt.Felt]("0x3b9aca0a"),
			l1GasPriceSTRK:  felt.NewUnsafeFromString[felt.Felt]("0x2b6fdb70"),
			l1DAGasPriceWEI: felt.NewUnsafeFromString[felt.Felt]("0x5265a14ef"),
			l1DAGasPriceFRI: felt.NewUnsafeFromString[felt.Felt]("0x3c0c00c87"),
		},
		{
			number:          64164,
			network:         utils.SepoliaIntegration,
			protocolVersion: "0.13.4",
			sig: &starknet.Signature{
				Signature: []*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0x2a1658d74c85266cec309b15fb623ae7b18854a8e08e84834067fd68f07d15a"),
					felt.NewUnsafeFromString[felt.Felt]("0x5304bd3e7151d87fabe5977a1d19c2cc9025cce27a2ce0b26a46633386add94"),
				},
			},
			l1GasPriceWEI:   felt.NewUnsafeFromString[felt.Felt]("0xcd62576b"),
			l1GasPriceSTRK:  felt.NewUnsafeFromString[felt.Felt]("0x17842b1d0815"),
			l1DAGasPriceWEI: felt.NewUnsafeFromString[felt.Felt]("0x31ea"),
			l1DAGasPriceFRI: felt.NewUnsafeFromString[felt.Felt]("0x5b70ba9"),
			l2GasPriceWEI:   felt.NewUnsafeFromString[felt.Felt]("0x15081"),
			l2GasPriceFRI:   felt.NewUnsafeFromString[felt.Felt]("0x268771a6"),
		},
	}

	ctx := t.Context()

	for _, test := range tests {
		t.Run(test.network.String()+" block number "+strconv.FormatUint(test.number, 10), func(t *testing.T) {
			client := feeder.NewTestClient(t, &test.network)

			response, err := client.Block(ctx, strconv.FormatUint(test.number, 10))
			require.NoError(t, err)
			block, err := sn2core.AdaptBlock(response, test.sig)
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
					assert.Equal(t, feederReceipt.ExecutionStatus == starknet.Reverted, block.Receipts[i].Reverted)
					assert.Equal(t, feederReceipt.RevertError, block.Receipts[i].RevertReason)
					if feederReceipt.ExecutionResources != nil {
						assert.Equal(t, (*core.DataAvailability)(feederReceipt.ExecutionResources.DataAvailability),
							block.Receipts[i].ExecutionResources.DataAvailability)
					}
				}
			}
			assert.Equal(t, expectedEventCount, block.EventCount)
			assert.Equal(t, test.protocolVersion, block.ProtocolVersion)

			if test.sig != nil {
				require.Len(t, block.Signatures, 1)
				assert.Equal(t, test.sig.Signature, block.Signatures[0])
			} else {
				assert.Empty(t, block.Signatures)
			}

			assert.Equal(t, test.l1GasPriceSTRK, block.L1GasPriceSTRK)
			assert.Equal(t, test.l1GasPriceWEI, block.L1GasPriceETH)
			if test.l1DAGasPriceFRI != nil {
				assert.Equal(t, test.l1DAGasPriceFRI, block.L1DataGasPrice.PriceInFri)
			}
			if test.l1DAGasPriceFRI != nil {
				assert.Equal(t, test.l1DAGasPriceWEI, block.L1DataGasPrice.PriceInWei)
			}
			if test.l2GasPriceFRI != nil {
				assert.Equal(t, test.l2GasPriceFRI, block.L2GasPrice.PriceInFri)
			}
			if test.l2GasPriceWEI != nil {
				assert.Equal(t, test.l2GasPriceWEI, block.L2GasPrice.PriceInWei)
			}
		})
	}
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client := feeder.NewTestClient(t, &utils.Mainnet)
	ctx := t.Context()

	for _, number := range numbers {
		t.Run("number "+strconv.FormatUint(number, 10), func(t *testing.T) {
			response, err := client.StateUpdate(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			feederUpdate, err := sn2core.AdaptStateUpdate(response)
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
				key := felt.NewUnsafeFromString[felt.Felt](keyStr)
				coreNonce := feederUpdate.StateDiff.Nonces[*key]
				assert.True(t, gw.Equal(coreNonce))
			}

			assert.Equal(t, len(response.StateDiff.DeployedContracts), len(feederUpdate.StateDiff.DeployedContracts))
			for idx, deployedContract := range response.StateDiff.DeployedContracts {
				gw := response.StateDiff.DeployedContracts[idx]
				coreDeployedContractClassHash := feederUpdate.StateDiff.DeployedContracts[*deployedContract.Address]
				assert.True(t, gw.ClassHash.Equal(coreDeployedContractClassHash))
			}

			assert.Equal(t, len(response.StateDiff.StorageDiffs), len(feederUpdate.StateDiff.StorageDiffs))
			for keyStr, diffs := range response.StateDiff.StorageDiffs {
				key := felt.NewUnsafeFromString[felt.Felt](keyStr)
				coreDiffs := feederUpdate.StateDiff.StorageDiffs[*key]
				assert.Equal(t, true, len(diffs) > 0)
				assert.Equal(t, len(diffs), len(coreDiffs))
				for _, diff := range diffs {
					assert.True(t, diff.Value.Equal(coreDiffs[*diff.Key]))
				}
			}
		})
	}

	t.Run("v0.11.0 state update", func(t *testing.T) {
		integClient := feeder.NewTestClient(t, &utils.Integration)

		t.Run("declared Cairo0 classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "283746")
			require.NoError(t, err)
			update, err := sn2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.DeclaredV0Classes)
			assert.Empty(t, update.StateDiff.DeclaredV1Classes)
			assert.Empty(t, update.StateDiff.ReplacedClasses)
		})

		t.Run("declared Cairo1 classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "283364")
			require.NoError(t, err)
			update, err := sn2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.Empty(t, update.StateDiff.DeclaredV0Classes)
			assert.NotEmpty(t, update.StateDiff.DeclaredV1Classes)
			assert.Empty(t, update.StateDiff.ReplacedClasses)
		})

		t.Run("replaced classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "283428")
			require.NoError(t, err)
			update, err := sn2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.Empty(t, update.StateDiff.DeclaredV0Classes)
			assert.Empty(t, update.StateDiff.DeclaredV1Classes)
			assert.NotEmpty(t, update.StateDiff.ReplacedClasses)
		})
	})
}

func TestClassV0(t *testing.T) {
	classHashes := []string{
		"0x7db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909",
		"0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
		"0x78401746828463e2c3f92ebb261fc82f7d4d4c8d9a80a356c44580dab124cb0",
		"0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43",
	}

	client := feeder.NewTestClient(t, &utils.Sepolia)
	ctx := t.Context()

	for _, hashString := range classHashes {
		t.Run("hash "+hashString, func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.Felt](hashString)
			response, err := client.ClassDefinition(ctx, hash)
			require.NoError(t, err)
			classGeneric, err := sn2core.AdaptDeprecatedCairoClass(response.DeprecatedCairo)
			require.NoError(t, err)
			class, ok := classGeneric.(*core.DeprecatedCairoClass)
			require.True(t, ok)

			for i, v := range response.DeprecatedCairo.EntryPoints.External {
				assert.Equal(t, v.Selector, class.Externals[i].Selector)
				assert.Equal(t, v.Offset, class.Externals[i].Offset)
			}
			assert.Equal(t, len(response.DeprecatedCairo.EntryPoints.External), len(class.Externals))

			for i, v := range response.DeprecatedCairo.EntryPoints.L1Handler {
				assert.Equal(t, v.Selector, class.L1Handlers[i].Selector)
				assert.Equal(t, v.Offset, class.L1Handlers[i].Offset)
			}
			assert.Equal(t, len(response.DeprecatedCairo.EntryPoints.L1Handler), len(class.L1Handlers))

			for i, v := range response.DeprecatedCairo.EntryPoints.Constructor {
				assert.Equal(t, v.Selector, class.Constructors[i].Selector)
				assert.Equal(t, v.Offset, class.Constructors[i].Offset)
			}
			assert.Equal(t, len(response.DeprecatedCairo.EntryPoints.Constructor), len(class.Constructors))

			assert.NotEmpty(t, class.Program)
		})
	}
}

func TestTransaction(t *testing.T) {
	clientGoerli := feeder.NewTestClient(t, &utils.Goerli)
	clientMainnet := feeder.NewTestClient(t, &utils.Mainnet)
	ctx := t.Context()

	t.Run("invoke transaction", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0x7e3a229febf47c6edfd96582d9476dd91a58a5ba3df4553ae448a14a2f132d9")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := sn2core.AdaptTransaction(responseTx)
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0x15b51c2f4880b1e7492d30ada7254fc59c09adde636f37eb08cdadbd9dabebb")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := sn2core.AdaptTransaction(responseTx)
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := sn2core.AdaptTransaction(responseTx)
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0x6eab8252abfc9bbfd72c8d592dde4018d07ce467c5ce922519d7142fcab203f")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := sn2core.AdaptTransaction(responseTx)
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := sn2core.AdaptTransaction(responseTx)
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

func TestTransactionV3(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	ctx := t.Context()

	tests := map[string]core.Transaction{
		// https://external.integration.starknet.io/feeder_gateway/get_transaction?transactionHash=0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd
		"invoke": &core.InvokeTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd"),
			Version:         new(core.TransactionVersion).SetUint64(3),
			TransactionSignature: []*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16"),
				felt.NewUnsafeFromString[felt.Felt]("0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"),
			},
			Nonce:       felt.NewUnsafeFromString[felt.Felt]("0xe97"),
			NonceDAMode: core.DAModeL1,
			FeeDAMode:   core.DAModeL1,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: {
					MaxAmount:       0x186a0,
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				core.ResourceL1DataGas: {
					MaxAmount:       0x186a0,
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				core.ResourceL2Gas: {
					MaxAmount:       0,
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:           0,
			PaymasterData: []*felt.Felt{},
			SenderAddress: felt.NewUnsafeFromString[felt.Felt]("0x3f6f3bc663aedc5285d6013cc3ffcbc4341d86ab488b8b68d297f8258793c41"),
			CallData: []*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x2"),
				felt.NewUnsafeFromString[felt.Felt]("0x450703c32370cf7ffff540b9352e7ee4ad583af143a361155f2b485c0c39684"),
				felt.NewUnsafeFromString[felt.Felt]("0x27c3334165536f239cfd400ed956eabff55fc60de4fb56728b6a4f6b87db01c"),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
				felt.NewUnsafeFromString[felt.Felt]("0x4"),
				felt.NewUnsafeFromString[felt.Felt]("0x4c312760dfd17a954cdd09e76aa9f149f806d88ec3e402ffaf5c4926f568a42"),
				felt.NewUnsafeFromString[felt.Felt]("0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf"),
				felt.NewUnsafeFromString[felt.Felt]("0x4"),
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x5"),
				felt.NewUnsafeFromString[felt.Felt]("0x450703c32370cf7ffff540b9352e7ee4ad583af143a361155f2b485c0c39684"),
				felt.NewUnsafeFromString[felt.Felt]("0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf"),
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x7fe4fd616c7fece1244b3616bb516562e230be8c9f29668b46ce0369d5ca829"),
				felt.NewUnsafeFromString[felt.Felt]("0x287acddb27a2f9ba7f2612d72788dc96a5b30e401fc1e8072250940e024a587"),
			},
			AccountDeploymentData: []*felt.Felt{},
		},
		// https://external.integration.starknet.io/feeder_gateway/get_transaction?transactionHash=0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3
		"declare": &core.DeclareTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3"),
			Version:         new(core.TransactionVersion).SetUint64(3),
			TransactionSignature: []*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x29a49dff154fede73dd7b5ca5a0beadf40b4b069f3a850cd8428e54dc809ccc"),
				felt.NewUnsafeFromString[felt.Felt]("0x429d142a17223b4f2acde0f5ecb9ad453e188b245003c86fab5c109bad58fc3"),
			},
			Nonce:       felt.NewUnsafeFromString[felt.Felt]("0x1"),
			NonceDAMode: core.DAModeL1,
			FeeDAMode:   core.DAModeL1,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: {
					MaxAmount:       0x186a0,
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x2540be400"),
				},
				core.ResourceL1DataGas: {
					MaxAmount:       0x186a0,
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x2540be400"),
				},
				core.ResourceL2Gas: {
					MaxAmount:       0,
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:                   0,
			PaymasterData:         []*felt.Felt{},
			SenderAddress:         felt.NewUnsafeFromString[felt.Felt]("0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50"),
			ClassHash:             felt.NewUnsafeFromString[felt.Felt]("0x5ae9d09292a50ed48c5930904c880dab56e85b825022a7d689cfc9e65e01ee7"),
			CompiledClassHash:     felt.NewUnsafeFromString[felt.Felt]("0x1add56d64bebf8140f3b8a38bdf102b7874437f0c861ab4ca7526ec33b4d0f8"),
			AccountDeploymentData: []*felt.Felt{},
		},
		// https://external.integration.starknet.io/feeder_gateway/get_transaction?transactionHash=0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0
		"deploy account": &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     felt.NewUnsafeFromString[felt.Felt]("0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0"),
				Version:             new(core.TransactionVersion).SetUint64(3),
				ContractAddress:     felt.NewUnsafeFromString[felt.Felt]("0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50"),
				ContractAddressSalt: new(felt.Felt),
				ClassHash:           felt.NewUnsafeFromString[felt.Felt]("0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738"),
				ConstructorCallData: []*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0x5cd65f3d7daea6c63939d659b8473ea0c5cd81576035a4d34e52fb06840196c"),
				},
			},
			Nonce:       new(felt.Felt),
			NonceDAMode: core.DAModeL1,
			FeeDAMode:   core.DAModeL1,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: {
					MaxAmount:       0x186a0,
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				core.ResourceL1DataGas: {
					MaxAmount:       0x186a0,
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				core.ResourceL2Gas: {
					MaxAmount:       0,
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			TransactionSignature: []*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x6d756e754793d828c6c1a89c13f7ec70dbd8837dfeea5028a673b80e0d6b4ec"),
				felt.NewUnsafeFromString[felt.Felt]("0x4daebba599f860daee8f6e100601d98873052e1c61530c630cc4375c6bd48e3"),
			},
			Tip:           0,
			PaymasterData: []*felt.Felt{},
		},
	}

	for description, want := range tests {
		t.Run(description, func(t *testing.T) {
			status, err := client.Transaction(ctx, want.Hash())
			require.NoError(t, err)
			tx, err := sn2core.AdaptTransaction(status.Transaction)
			require.NoError(t, err)
			require.Equal(t, want, tx)
		})
	}
}

func TestClassV1(t *testing.T) {
	tests := []struct {
		name                  string
		classHash             string
		network               *utils.Network
		expectedSierraVersion string
	}{
		{
			name:                  "cairo one class adapt sierra version 0.1.0",
			classHash:             "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
			network:               &utils.Integration,
			expectedSierraVersion: "0.1.0",
		},
		{
			name:                  "cairo one class adapt sierra version 1.6.0",
			classHash:             "0x03cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b",
			network:               &utils.Sepolia,
			expectedSierraVersion: "1.6.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := feeder.NewTestClient(t, test.network)
			classHash := felt.NewUnsafeFromString[felt.Felt](test.classHash)

			feederClass, err := client.ClassDefinition(t.Context(), classHash)
			require.NoError(t, err)
			compiled, err := client.CasmClassDefinition(t.Context(), classHash)
			require.NoError(t, err)

			v1Class, err := sn2core.AdaptSierraClass(feederClass.Sierra, compiled)
			require.NoError(t, err)

			assert.Equal(t, feederClass.Sierra.Abi, v1Class.Abi)
			assert.Equal(t, feederClass.Sierra.Program, v1Class.Program)
			assert.Equal(t, feederClass.Sierra.Version, v1Class.SemanticVersion)
			assert.Equal(t, compiled.Prime, "0x"+v1Class.Compiled.Prime.Text(felt.Base16))
			assert.Equal(t, compiled.Bytecode, v1Class.Compiled.Bytecode)
			assert.Equal(t, compiled.Hints, v1Class.Compiled.Hints)
			assert.Equal(t, compiled.CompilerVersion, v1Class.Compiled.CompilerVersion)
			assert.Equal(t, len(compiled.EntryPoints.External), len(v1Class.Compiled.External))
			assert.Equal(
				t,
				len(compiled.EntryPoints.Constructor),
				len(v1Class.Compiled.Constructor),
			)
			assert.Equal(t,
				len(compiled.EntryPoints.L1Handler),
				len(v1Class.Compiled.L1Handler),
			)

			assert.Equal(
				t,
				len(feederClass.Sierra.EntryPoints.External),
				len(v1Class.EntryPoints.External),
			)
			for i, v := range feederClass.Sierra.EntryPoints.External {
				assert.Equal(t, v.Selector, v1Class.EntryPoints.External[i].Selector)
				assert.Equal(t, v.Index, v1Class.EntryPoints.External[i].Index)
			}

			assert.Equal(
				t,
				len(feederClass.Sierra.EntryPoints.Constructor),
				len(v1Class.EntryPoints.Constructor),
			)
			for i, v := range feederClass.Sierra.EntryPoints.Constructor {
				assert.Equal(t, v.Selector, v1Class.EntryPoints.Constructor[i].Selector)
				assert.Equal(t, v.Index, v1Class.EntryPoints.Constructor[i].Index)
			}

			assert.Equal(
				t,
				len(feederClass.Sierra.EntryPoints.L1Handler),
				len(v1Class.EntryPoints.L1Handler),
			)
			for i, v := range feederClass.Sierra.EntryPoints.L1Handler {
				assert.Equal(t, v.Selector, v1Class.EntryPoints.L1Handler[i].Selector)
				assert.Equal(t, v.Index, v1Class.EntryPoints.L1Handler[i].Index)
			}

			assert.Equal(t, test.expectedSierraVersion, v1Class.SierraVersion())
		})
	}

	t.Run("sierra class is empty", func(t *testing.T) {
		snClass := starknet.SierraClass{}
		class, err := sn2core.AdaptSierraClass(&snClass, nil)
		require.Nil(t, class)
		require.Contains(t, "sierra program size is too small", err.Error())
	})

	t.Run("sierra class doesn't have the minimum size", func(t *testing.T) {
		snClass := starknet.SierraClass{
			Program: []*felt.Felt{
				new(felt.Felt), // this value doesn't matter as long as their different from `SierraVersion010`
				new(felt.Felt),
			},
		}
		class, err := sn2core.AdaptSierraClass(&snClass, nil)
		require.Nil(t, class)
		require.Contains(t, "sierra program size is too small", err.Error())
	})
}

func TestAdaptCompiledClass(t *testing.T) {
	result, err := sn2core.AdaptCompiledClass(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAdaptPreConfirmed(t *testing.T) {
	n := &utils.SepoliaIntegration
	client := feeder.NewTestClient(t, n)
	emptyPreConfirmedBlock := uint64(1201960)
	blockWithCandidates := uint64(1204672)
	blockWithNoCandidates := uint64(1204673)
	blockFullOfCandidates := uint64(1204674)
	blocksWithRandomCandidateOrder := uint64(1204675)

	type preConfirmedTest struct {
		description string
		blockNumber uint64
	}

	tests := []preConfirmedTest{
		{
			description: "PreConfirmedBlock when there is no candidates",
			blockNumber: blockWithNoCandidates,
		},
		{
			description: "PreConfirmedBlock with candidates",
			blockNumber: blockWithCandidates,
		},
		{
			description: "PreConfirmedBlock when full of candidates",
			blockNumber: blockFullOfCandidates,
		},
		{
			description: "PreConfirmedBlock when empty",
			blockNumber: emptyPreConfirmedBlock,
		},
		{
			description: "PreConfirmedBlock with candidate in between preconfirmed txns",
			blockNumber: blocksWithRandomCandidateOrder,
		},
	}

	for _, test := range tests {
		response, err := client.PreConfirmedBlock(
			t.Context(),
			strconv.FormatUint(test.blockNumber, 10),
		)
		require.NoError(t, err)

		expectedEventCount, expectedPreConfirmedTxCount := countEventsAndTxs(response.Receipts)
		expectedCandidateCount := len(response.Transactions) - expectedPreConfirmedTxCount
		expectedReceipts := getPreconfirmedReceipts(response.Receipts)

		adapted, err := sn2core.AdaptPreConfirmedBlock(response, test.blockNumber)
		require.NoError(t, err)

		assertPreConfirmedBlockBasics(
			t,
			&adapted,
			test.blockNumber,
			response,
			expectedPreConfirmedTxCount,
			expectedCandidateCount,
			expectedEventCount,
		)
		assertPreConfirmedBlockReceipts(
			t,
			expectedReceipts,
			adapted.Block.Receipts,
			expectedPreConfirmedTxCount,
		)
		assertPreConfirmedBlockGasPrices(t, response, adapted.Block)
		assertCandidateTxs(t, response, adapted.CandidateTxs)
		assertStateDiffs(t, response, adapted.TransactionStateDiffs)
	}
}

func getPreconfirmedReceipts(
	receipts []*starknet.TransactionReceipt,
) []*starknet.TransactionReceipt {
	var filtered []*starknet.TransactionReceipt
	for _, r := range receipts {
		if r != nil {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func assertPreConfirmedBlockBasics(
	t *testing.T,
	preConfirmed *core.PreConfirmed,
	blockNum uint64,
	response *starknet.PreConfirmedBlock,
	expectedTxCount int,
	expectedCandidateCount int,
	expectedEventCount uint64,
) {
	assert.Equal(t, blockNum, preConfirmed.Block.Number)
	assert.NotNil(t, preConfirmed.Block.EventsBloom)
	assert.Empty(t, preConfirmed.Block.Hash)
	assert.Empty(t, preConfirmed.Block.ParentHash)
	assert.Empty(t, preConfirmed.Block.GlobalStateRoot)
	assert.Empty(t, preConfirmed.Block.Signatures)
	assert.Empty(t, preConfirmed.StateUpdate.NewRoot)
	assert.Empty(t, preConfirmed.StateUpdate.BlockHash)
	assert.Equal(t, response.Timestamp, preConfirmed.Block.Timestamp)
	assert.Equal(t, expectedTxCount, len(preConfirmed.Block.Transactions))
	assert.Equal(t, expectedCandidateCount, len(preConfirmed.CandidateTxs))
	assert.Equal(t, uint64(expectedTxCount), preConfirmed.Block.TransactionCount)
	assert.Equal(t, expectedEventCount, preConfirmed.Block.EventCount)
	assert.Equal(t, response.Version, preConfirmed.Block.ProtocolVersion)
	assert.Equal(t, response.L1GasPrice.PriceInFri, preConfirmed.Block.L1GasPriceSTRK)
	assert.Equal(t, response.L1GasPrice.PriceInWei, preConfirmed.Block.L1GasPriceETH)
}

func assertCandidateTxs(
	t *testing.T,
	response *starknet.PreConfirmedBlock,
	candidateTxs []core.Transaction,
) {
	t.Helper()

	candID := 0
	for i := range response.Transactions {
		if sn2core.IsCandidateTx(response, i) {
			adaptedTx, err := sn2core.AdaptTransaction(&response.Transactions[i])
			require.NoError(t, err)
			assert.Equal(t, adaptedTx, candidateTxs[candID])
			candID++
		}
	}
	assert.Equal(t, len(candidateTxs), candID)
}

func assertStateDiffs(
	t *testing.T,
	response *starknet.PreConfirmedBlock,
	stateDiffs []*core.StateDiff,
) {
	t.Helper()

	preID := 0
	for i := range response.Transactions {
		if !sn2core.IsCandidateTx(response, i) {
			adaptedStateDiff, err := sn2core.AdaptStateDiff(response.TransactionStateDiffs[i])
			require.NoError(t, err)
			assert.Equal(t, &adaptedStateDiff, stateDiffs[preID])
			preID++
		}
	}
	assert.Equal(t, len(stateDiffs), preID)
}

func assertPreConfirmedBlockGasPrices(t *testing.T, response *starknet.PreConfirmedBlock, block *core.Block) {
	t.Helper()
	assert.Equal(t, response.L1DataGasPrice.PriceInFri, block.L1DataGasPrice.PriceInFri)
	assert.Equal(t, response.L1DataGasPrice.PriceInWei, block.L1DataGasPrice.PriceInWei)
	assert.Equal(t, response.L2GasPrice.PriceInFri, block.L2GasPrice.PriceInFri)
	assert.Equal(t, response.L2GasPrice.PriceInWei, block.L2GasPrice.PriceInWei)
}

func countEventsAndTxs(receipts []*starknet.TransactionReceipt) (uint64, int) {
	var evCount uint64
	txCount := 0
	for _, r := range receipts {
		if r == nil {
			continue
		}
		evCount += uint64(len(r.Events))
		txCount++
	}
	return evCount, txCount
}

func assertPreConfirmedBlockReceipts(
	t *testing.T,
	expectedReceipts []*starknet.TransactionReceipt,
	blockReceipts []*core.TransactionReceipt,
	expectedPreConfirmedTxCount int,
) {
	t.Helper()
	if assert.Equal(t, expectedPreConfirmedTxCount, len(blockReceipts)) {
		for i, actual := range blockReceipts {
			expected := expectedReceipts[i]
			assert.Equal(t, expected.ExecutionStatus == starknet.Reverted, actual.Reverted)
			assert.Equal(t, expected.RevertError, actual.RevertReason)
			if expected.ExecutionResources != nil {
				assert.Equal(
					t,
					(*core.DataAvailability)(expected.ExecutionResources.DataAvailability),
					actual.ExecutionResources.DataAvailability,
				)
			}
		}
	}
}
