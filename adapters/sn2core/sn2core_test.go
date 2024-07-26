package sn2core_test

import (
	"context"
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
		gasPriceWEI     *felt.Felt
		gasPriceSTRK    *felt.Felt
		l1DAGasPriceWEI *felt.Felt
		l1DAGasPriceFRI *felt.Felt
	}{
		{
			number:      147,
			network:     utils.Mainnet,
			gasPriceWEI: &felt.Zero,
		},
		{
			number:          11817,
			protocolVersion: "0.10.1",
			network:         utils.Mainnet,
			gasPriceWEI:     utils.HexToFelt(t, "0x27ad16775"),
		},
		{
			number:          4850,
			protocolVersion: "0.12.3",
			network:         utils.Sepolia,
			sig: &starknet.Signature{
				Signature: []*felt.Felt{utils.HexToFelt(t, "0x44"), utils.HexToFelt(t, "0x37")},
			},
			gasPriceWEI:  utils.HexToFelt(t, "0x80197ea0"),
			gasPriceSTRK: utils.HexToFelt(t, "0x0"),
		},
		{
			number:          56377,
			network:         utils.Sepolia,
			protocolVersion: "0.13.1",
			sig: &starknet.Signature{
				Signature: []*felt.Felt{
					utils.HexToFelt(t, "0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16"),
					utils.HexToFelt(t, "0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"),
				},
			},
			gasPriceWEI:  utils.HexToFelt(t, "0x4a817c800"),
			gasPriceSTRK: utils.HexToFelt(t, "0x1d1a94a20000"),
		},
		{
			number:          16350,
			network:         utils.SepoliaIntegration,
			protocolVersion: "0.13.1",
			sig: &starknet.Signature{
				Signature: []*felt.Felt{
					utils.HexToFelt(t, "0x7685fbcd4bacae7554ad17c6962221143911d894d7b8794d29234623f392562"),
					utils.HexToFelt(t, "0x343e605de3957e664746ba8ef82f2b0f9d53cda3d75dcb078290b8edd010165"),
				},
			},
			gasPriceWEI:     utils.HexToFelt(t, "0x3b9aca10"),
			gasPriceSTRK:    utils.HexToFelt(t, "0x17882b6aa74"),
			l1DAGasPriceWEI: utils.HexToFelt(t, "0x716a8f6dd"),
			l1DAGasPriceFRI: utils.HexToFelt(t, "0x2cc6d7f596e1"),
		},
	}

	ctx := context.Background()

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

			assert.Equal(t, test.gasPriceSTRK, block.GasPriceSTRK)
			assert.Equal(t, test.gasPriceWEI, block.GasPrice)
			if test.l1DAGasPriceFRI != nil {
				assert.Equal(t, test.l1DAGasPriceFRI, block.L1DataGasPrice.PriceInFri)
			}
			if test.l1DAGasPriceFRI != nil {
				assert.Equal(t, test.l1DAGasPriceWEI, block.L1DataGasPrice.PriceInWei)
			}
		})
	}
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client := feeder.NewTestClient(t, &utils.Mainnet)
	ctx := context.Background()

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
				key := utils.HexToFelt(t, keyStr)
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
				key := utils.HexToFelt(t, keyStr)
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
		integClient := feeder.NewTestClient(t, &utils.Sepolia)

		t.Run("declared Cairo0 classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "18")
			require.NoError(t, err)
			update, err := sn2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.Empty(t, update.StateDiff.DeclaredV0Classes)
			assert.NotEmpty(t, update.StateDiff.DeclaredV1Classes)
			assert.Empty(t, update.StateDiff.ReplacedClasses)
		})

		t.Run("declared Cairo1 classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "7")
			require.NoError(t, err)
			update, err := sn2core.AdaptStateUpdate(feederUpdate)
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.DeclaredV0Classes)
			assert.NotEmpty(t, update.StateDiff.DeclaredV1Classes)
			assert.Empty(t, update.StateDiff.ReplacedClasses)
		})

		t.Run("replaced classes", func(t *testing.T) {
			feederUpdate, err := integClient.StateUpdate(ctx, "6500")
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
	ctx := context.Background()

	for _, hashString := range classHashes {
		t.Run("hash "+hashString, func(t *testing.T) {
			hash := utils.HexToFelt(t, hashString)
			response, err := client.ClassDefinition(ctx, hash)
			require.NoError(t, err)
			classGeneric, err := sn2core.AdaptCairo0Class(response.V0)
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
	clientMainnet := feeder.NewTestClient(t, &utils.Mainnet)
	ctx := context.Background()

	t.Run("invoke transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x6c40890743aa220b10e5ee68cef694c5c23cc2defd0dbdf5546e687f9982ab1")
		response, err := clientMainnet.Transaction(ctx, hash)
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
		hash := utils.HexToFelt(t, "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee")
		response, err := clientMainnet.Transaction(ctx, hash)
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
		hash := utils.HexToFelt(t, "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2")
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
		hash := utils.HexToFelt(t, "0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4")
		response, err := clientMainnet.Transaction(ctx, hash)
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
		hash := utils.HexToFelt(t, "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8")
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
	client := feeder.NewTestClient(t, &utils.Sepolia)
	ctx := context.Background()

	tests := map[string]core.Transaction{
		"invoke": &core.InvokeTransaction{
			TransactionHash: utils.HexToFelt(t, "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164"),
			Version:         new(core.TransactionVersion).SetUint64(3),
			TransactionSignature: []*felt.Felt{
				utils.HexToFelt(t, "0x01"),
				utils.HexToFelt(t, "0x4235b7a9cad6024cbb3296325e23b2a03d34a95c3ee3d5c10e2b6076c257d77"),
				utils.HexToFelt(t, "0x439de4b0c238f624c14c2619aa9d190c6c1d17f6556af09f1697cfe74f192fc"),
			},
			Nonce:       utils.HexToFelt(t, "0x8"),
			NonceDAMode: core.DAModeL1,
			FeeDAMode:   core.DAModeL1,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: {
					MaxAmount:       utils.HexToUint64(t, "0xa0"),
					MaxPricePerUnit: utils.HexToFelt(t, "0xe91444530acc"),
				},
				core.ResourceL2Gas: {
					MaxAmount:       0,
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:           0,
			PaymasterData: []*felt.Felt{},
			SenderAddress: utils.HexToFelt(t, "0x6247aaebf5d2ff56c35cce1585bf255963d94dd413a95020606d523c8c7f696"),
			CallData: []*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x19c92fa87f4d5e3be25c3dd6a284f30282a07e87cd782f5fd387b82c8142017"),
				utils.HexToFelt(t, "0x3059098e39fbb607bc96a8075eb4d17197c3a6c797c166470997571e6fa5b17"),
				utils.HexToFelt(t, "0x0"),
			},
			AccountDeploymentData: []*felt.Felt{},
		},
		"declare": &core.DeclareTransaction{
			TransactionHash: utils.HexToFelt(t, "0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491"),
			Version:         new(core.TransactionVersion).SetUint64(3),
			TransactionSignature: []*felt.Felt{
				utils.HexToFelt(t, "0x5be36745b03aaeb76712c68869f944f7c711f9e734763b8d0b4e5b834408ea4"),
				utils.HexToFelt(t, "0x66c9dba8bb26ada30cf3a393a6c26bfd3a40538f19b3b4bfb57c7507962ae79"),
			},
			Nonce:       utils.HexToFelt(t, "0x3"),
			NonceDAMode: core.DAModeL1,
			FeeDAMode:   core.DAModeL1,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: {
					MaxAmount:       utils.HexToUint64(t, "0x1f40"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x5af3107a4000"),
				},
				core.ResourceL2Gas: {
					MaxAmount:       0,
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:                   0,
			PaymasterData:         []*felt.Felt{},
			SenderAddress:         utils.HexToFelt(t, "0x6aac79bb6c90e1e41c33cd20c67c0281c4a95f01b4e15ad0c3b53fcc6010cf8"),
			ClassHash:             utils.HexToFelt(t, "0x2404dffbfe2910bd921f5935e628c01e457629fc779420a03b7e5e507212f36"),
			CompiledClassHash:     utils.HexToFelt(t, "0x5047109bf7eb550c5e6b0c37714f6e0db4bb8b5b227869e0797ecfc39240aa7"),
			AccountDeploymentData: []*felt.Felt{},
		},
		"deploy account": &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     utils.HexToFelt(t, "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f"),
				Version:             new(core.TransactionVersion).SetUint64(3),
				ContractAddress:     utils.HexToFelt(t, "0x7ac1f9f2dde09f938e7ace23009160bf4b2f48a69363983abc1f6d51cb39e37"),
				ContractAddressSalt: utils.HexToFelt(t, "0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2"),
				ClassHash:           utils.HexToFelt(t, "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b"),
				ConstructorCallData: []*felt.Felt{
					utils.HexToFelt(t, "0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2"),
					utils.HexToFelt(t, "0x0"),
				},
			},
			Nonce:       new(felt.Felt),
			NonceDAMode: core.DAModeL1,
			FeeDAMode:   core.DAModeL1,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: {
					MaxAmount:       utils.HexToUint64(t, "0x1b52"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x15416c61bfea"),
				},
				core.ResourceL2Gas: {
					MaxAmount:       0,
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			TransactionSignature: []*felt.Felt{
				utils.HexToFelt(t, "0x79ec88c0f655e07f49a66bc6d4d9e696cf578addf6a0538f81dc3b47ca66c64"),
				utils.HexToFelt(t, "0x78d3f2549f6f5b8533730a0f4f76c4277bc1b358f805d7cf66414289ce0a46d"),
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
	client := feeder.NewTestClient(t, &utils.Mainnet)

	classHash := utils.HexToFelt(t, "0x21c2e8a87c431e8d3e89ecd1a40a0674ef533cce5a1f6c44ba9e60d804ecad2")

	feederClass, err := client.ClassDefinition(context.Background(), classHash)
	require.NoError(t, err)
	compiled, err := client.CompiledClassDefinition(context.Background(), classHash)
	require.NoError(t, err)

	v1Class, err := sn2core.AdaptCairo1Class(feederClass.V1, compiled)
	require.NoError(t, err)

	assert.Equal(t, feederClass.V1.Abi, v1Class.Abi)
	assert.Equal(t, feederClass.V1.Program, v1Class.Program)
	assert.Equal(t, feederClass.V1.Version, v1Class.SemanticVersion)
	assert.Equal(t, compiled.Prime, "0x"+v1Class.Compiled.Prime.Text(felt.Base16))
	assert.Equal(t, compiled.Bytecode, v1Class.Compiled.Bytecode)
	assert.Equal(t, compiled.Hints, v1Class.Compiled.Hints)
	assert.Equal(t, compiled.CompilerVersion, v1Class.Compiled.CompilerVersion)
	assert.Equal(t, len(compiled.EntryPoints.External), len(v1Class.Compiled.External))
	assert.Equal(t, len(compiled.EntryPoints.Constructor), len(v1Class.Compiled.Constructor))
	assert.Equal(t, len(compiled.EntryPoints.L1Handler), len(v1Class.Compiled.L1Handler))

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

func TestAdaptCompiledClass(t *testing.T) {
	result, err := sn2core.AdaptCompiledClass(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}
