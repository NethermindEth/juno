package feeder_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBlockByNumber(t *testing.T) {
	tests := []struct {
		number          uint64
		protocolVersion string
	}{
		{
			number: 147,
		},
		{
			number:          11817,
			protocolVersion: "0.10.1",
		},
	}

	client, serverClose := feeder.NewTestClient(utils.MAINNET)
	defer serverClose()
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, test := range tests {
		t.Run("mainnet block number "+strconv.FormatUint(test.number, 10), func(t *testing.T) {
			response, err := client.Block(ctx, test.number)
			require.NoError(t, err)
			block, err := adapter.BlockByNumber(ctx, test.number)
			require.NoError(t, err)

			expectedEventCount := uint64(0)
			for _, r := range response.Receipts {
				expectedEventCount = expectedEventCount + uint64(len(r.Events))
			}

			assert.True(t, block.Hash.Equal(response.Hash))
			assert.True(t, block.ParentHash.Equal(response.ParentHash))
			assert.Equal(t, response.Number, block.Number)
			assert.True(t, block.GlobalStateRoot.Equal(response.StateRoot))
			assert.Equal(t, response.Timestamp, block.Timestamp)
			assert.Equal(t, len(response.Transactions), len(block.Transactions))
			assert.Equal(t, uint64(len(response.Transactions)), block.TransactionCount)
			assert.Equal(t, len(response.Receipts), len(block.Receipts))
			assert.Equal(t, expectedEventCount, block.EventCount)
			assert.Equal(t, test.protocolVersion, block.ProtocolVersion)
			assert.Nil(t, block.ExtraData)
		})
	}
}

func hexToFelt(t *testing.T, hex string) *felt.Felt {
	f, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	return f
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client, serverClose := feeder.NewTestClient(utils.MAINNET)
	defer serverClose()
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, number := range numbers {
		t.Run("number "+strconv.FormatUint(number, 10), func(t *testing.T) {
			response, err := client.StateUpdate(ctx, number)
			require.NoError(t, err)
			feederUpdate, err := adapter.StateUpdate(ctx, number)
			require.NoError(t, err)

			if assert.NoError(t, err) {
				assert.True(t, response.NewRoot.Equal(feederUpdate.NewRoot))
				assert.True(t, response.OldRoot.Equal(feederUpdate.OldRoot))
				assert.True(t, response.BlockHash.Equal(feederUpdate.BlockHash))

				assert.Equal(t, len(response.StateDiff.DeclaredContracts), len(feederUpdate.StateDiff.DeclaredClasses))
				for idx := range response.StateDiff.DeclaredContracts {
					resp := response.StateDiff.DeclaredContracts[idx]
					core := feederUpdate.StateDiff.DeclaredClasses[idx]
					assert.True(t, resp.Equal(core))
				}

				assert.Equal(t, len(response.StateDiff.Nonces), len(feederUpdate.StateDiff.Nonces))
				for keyStr, gw := range response.StateDiff.Nonces {
					key, _ := new(felt.Felt).SetString(keyStr)
					core := feederUpdate.StateDiff.Nonces[*key]
					assert.True(t, gw.Equal(core))
				}

				assert.Equal(t, len(response.StateDiff.DeployedContracts), len(feederUpdate.StateDiff.DeployedContracts))
				for idx := range response.StateDiff.DeployedContracts {
					gw := response.StateDiff.DeployedContracts[idx]
					core := feederUpdate.StateDiff.DeployedContracts[idx]
					assert.True(t, gw.ClassHash.Equal(core.ClassHash))
					assert.True(t, gw.Address.Equal(core.Address))
				}

				assert.Equal(t, len(response.StateDiff.StorageDiffs), len(feederUpdate.StateDiff.StorageDiffs))
				for keyStr, diffs := range response.StateDiff.StorageDiffs {
					key, _ := new(felt.Felt).SetString(keyStr)
					coreDiffs := feederUpdate.StateDiff.StorageDiffs[*key]
					assert.Equal(t, true, len(diffs) > 0)
					assert.Equal(t, len(diffs), len(coreDiffs))
					for idx := range diffs {
						assert.Equal(t, true, diffs[idx].Key.Equal(coreDiffs[idx].Key))
						assert.Equal(t, true, diffs[idx].Value.Equal(coreDiffs[idx].Value))
					}
				}
			}
		})
	}
}

func TestClass(t *testing.T) {
	classHashes := []string{
		"0x79e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
		"0x1924aa4b0bedfd884ea749c7231bafd91650725d44c91664467ffce9bf478d0",
		"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		"0x56b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
	}

	client, serverClose := feeder.NewTestClient(utils.GOERLI)
	defer serverClose()
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, hashString := range classHashes {
		t.Run("hash "+hashString, func(t *testing.T) {
			hash := hexToFelt(t, hashString)
			response, err := client.ClassDefinition(ctx, hash)
			require.NoError(t, err)
			class, err := adapter.Class(ctx, hash)
			require.NoError(t, err)

			for i, v := range response.EntryPoints.External {
				assert.Equal(t, v.Selector, class.Externals[i].Selector)
				assert.Equal(t, v.Offset, class.Externals[i].Offset)
			}
			assert.Equal(t, len(response.EntryPoints.External), len(class.Externals))

			for i, v := range response.EntryPoints.L1Handler {
				assert.Equal(t, v.Selector, class.L1Handlers[i].Selector)
				assert.Equal(t, v.Offset, class.L1Handlers[i].Offset)
			}
			assert.Equal(t, len(response.EntryPoints.L1Handler), len(class.L1Handlers))

			for i, v := range response.EntryPoints.Constructor {
				assert.Equal(t, v.Selector, class.Constructors[i].Selector)
				assert.Equal(t, v.Offset, class.Constructors[i].Offset)
			}
			assert.Equal(t, len(response.EntryPoints.Constructor), len(class.Constructors))

			for i, v := range response.Program.Builtins {
				assert.Equal(t, new(felt.Felt).SetBytes([]byte(v)), class.Builtins[i])
			}
			assert.Equal(t, len(response.Program.Builtins), len(class.Builtins))

			for i, v := range response.Program.Data {
				expected, err := new(felt.Felt).SetString(v)
				assert.NoError(t, err)
				assert.Equal(t, expected, class.Bytecode[i])
			}
			assert.Equal(t, len(response.Program.Data), len(class.Bytecode))

			programHash, err := feeder.ProgramHash(response)
			assert.NoError(t, err)
			assert.Equal(t, programHash, class.ProgramHash)
		})
	}
}

func TestTransaction(t *testing.T) {
	clientGoerli, serverClose := feeder.NewTestClient(utils.GOERLI)
	defer serverClose()
	adapterGoerli := adaptfeeder.New(clientGoerli)

	clientMainnet, serverClose := feeder.NewTestClient(utils.MAINNET)
	defer serverClose()
	adapterMainnet := adaptfeeder.New(clientMainnet)

	ctx := context.Background()

	t.Run("invoke transaction", func(t *testing.T) {
		hash := hexToFelt(t, "0x7e3a229febf47c6edfd96582d9476dd91a58a5ba3df4553ae448a14a2f132d9")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		invokeTx, ok := txn.(*core.InvokeTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, invokeTx.Hash())
		assert.Equal(t, responseTx.ContractAddress, invokeTx.ContractAddress)
		assert.Equal(t, responseTx.EntryPointSelector, invokeTx.EntryPointSelector)
		assert.Equal(t, responseTx.Nonce, invokeTx.Nonce)
		assert.Equal(t, responseTx.CallData, invokeTx.CallData)
		assert.Equal(t, responseTx.Signature, invokeTx.Signature())
		assert.Equal(t, responseTx.MaxFee, invokeTx.MaxFee)
		assert.Equal(t, responseTx.Version, invokeTx.Version)
	})

	t.Run("deploy transaction", func(t *testing.T) {
		hash := hexToFelt(t, "0x15b51c2f4880b1e7492d30ada7254fc59c09adde636f37eb08cdadbd9dabebb")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		deployTx, ok := txn.(*core.DeployTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, deployTx.Hash())
		assert.Equal(t, responseTx.ContractAddressSalt, deployTx.ContractAddressSalt)
		assert.Equal(t, responseTx.ContractAddress, deployTx.ContractAddress)
		assert.Equal(t, responseTx.ClassHash, deployTx.ClassHash)
		assert.Equal(t, responseTx.ConstructorCallData, deployTx.ConstructorCallData)
		assert.Equal(t, responseTx.Version, deployTx.Version)
	})

	t.Run("deploy account transaction", func(t *testing.T) {
		hash := hexToFelt(t, "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		deployAccountTx, ok := txn.(*core.DeployAccountTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, deployAccountTx.Hash())
		assert.Equal(t, responseTx.ContractAddressSalt, deployAccountTx.ContractAddressSalt)
		assert.Equal(t, responseTx.ContractAddress, deployAccountTx.ContractAddress)
		assert.Equal(t, responseTx.ClassHash, deployAccountTx.ClassHash)
		assert.Equal(t, responseTx.ConstructorCallData, deployAccountTx.ConstructorCallData)
		assert.Equal(t, responseTx.Version, deployAccountTx.Version)
		assert.Equal(t, responseTx.MaxFee, deployAccountTx.MaxFee)
		assert.Equal(t, responseTx.Signature, deployAccountTx.Signature())
		assert.Equal(t, responseTx.Nonce, deployAccountTx.Nonce)
	})

	t.Run("declare transaction", func(t *testing.T) {
		hash := hexToFelt(t, "0x6eab8252abfc9bbfd72c8d592dde4018d07ce467c5ce922519d7142fcab203f")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		declareTx, ok := txn.(*core.DeclareTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, declareTx.Hash())
		assert.Equal(t, responseTx.SenderAddress, declareTx.SenderAddress)
		assert.Equal(t, responseTx.Version, declareTx.Version)
		assert.Equal(t, responseTx.Nonce, declareTx.Nonce)
		assert.Equal(t, responseTx.MaxFee, declareTx.MaxFee)
		assert.Equal(t, responseTx.Signature, declareTx.Signature())
		assert.Equal(t, responseTx.ClassHash, declareTx.ClassHash)
	})

	t.Run("l1handler transaction", func(t *testing.T) {
		hash := hexToFelt(t, "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		l1HandlerTx, ok := txn.(*core.L1HandlerTransaction)
		require.True(t, ok)
		require.NoError(t, err)

		assert.Equal(t, responseTx.Hash, l1HandlerTx.Hash())
		assert.Equal(t, responseTx.ContractAddress, l1HandlerTx.ContractAddress)
		assert.Equal(t, responseTx.EntryPointSelector, l1HandlerTx.EntryPointSelector)
		assert.Equal(t, responseTx.Nonce, l1HandlerTx.Nonce)
		assert.Equal(t, responseTx.CallData, l1HandlerTx.CallData)
		assert.Equal(t, responseTx.Version, l1HandlerTx.Version)
	})
}
