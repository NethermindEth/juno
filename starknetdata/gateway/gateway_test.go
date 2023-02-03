package gateway

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/block_11817.json
	block11817Json []byte
	//go:embed testdata/mainnet_block_147.json
	block147Json []byte
	//go:embed testdata/class_0x1924aa4b0bedfd884ea749c7231bafd91650725d44c91664467ffce9bf478d0.json
	classJson []byte
	//go:embed testdata/invokeTx_0x7e3a229febf47c6edfd96582d9476dd91a58a5ba3df4553ae448a14a2f132d9.json
	invokeJson []byte
	//go:embed testdata/deployTx_0x15b51c2f4880b1e7492d30ada7254fc59c09adde636f37eb08cdadbd9dabebb.json
	deployJson []byte
	//go:embed testdata/declareTx_0x6eab8252abfc9bbfd72c8d592dde4018d07ce467c5ce922519d7142fcab203f.json
	declareJson []byte
)

func TestAdaptBlock(t *testing.T) {
	var response *clients.Block

	t.Run("mainnet block number 11817", func(t *testing.T) {
		err := json.Unmarshal(block11817Json, &response)
		require.NoError(t, err)

		block, err := AdaptBlock(response)
		require.NoError(t, err)

		assert.True(t, block.Hash.Equal(response.Hash))
		assert.True(t, block.ParentHash.Equal(response.ParentHash))
		assert.Equal(t, response.Number, block.Number)
		assert.True(t, block.GlobalStateRoot.Equal(response.StateRoot))
		assert.True(t, block.Timestamp.Equal(new(felt.Felt).SetUint64(response.Timestamp)))
		assert.Equal(t, new(felt.Felt).SetUint64(uint64(len(response.Transactions))), block.TransactionCount)
		assert.Equal(t, new(felt.Felt), block.ProtocolVersion)
		assert.Nil(t, block.ExtraData)
		// TODO test transaction commitment...?
		// TODO test event commitment and count
	})
	t.Run("mainnet block number 147", func(t *testing.T) {
		err := json.Unmarshal(block147Json, &response)
		require.NoError(t, err)

		block, err := AdaptBlock(response)
		require.NoError(t, err)

		assert.True(t, block.Hash.Equal(response.Hash))
		assert.True(t, block.ParentHash.Equal(response.ParentHash))
		assert.Equal(t, response.Number, block.Number)
		assert.True(t, block.GlobalStateRoot.Equal(response.StateRoot))
		assert.True(t, block.Timestamp.Equal(new(felt.Felt).SetUint64(response.Timestamp)))
		assert.Equal(t, new(felt.Felt).SetUint64(uint64(len(response.Transactions))), block.TransactionCount)
		assert.Equal(t, new(felt.Felt), block.ProtocolVersion)
		assert.Nil(t, block.ExtraData)
		// TODO test transaction commitment...?
		// TODO test event commitment and count
	})
	t.Run("error with unknown transaction", func(t *testing.T) {
		err := json.Unmarshal(block147Json, &response)
		require.NoError(t, err)

		response.Transactions[0].Type = "test"

		block, err := AdaptBlock(response)
		assert.Nil(t, block)
		assert.EqualError(t, err, "unknown transaction")
	})
}

func TestAdaptStateUpdate(t *testing.T) {
	jsonData := []byte(`{
  "block_hash": "0x3",
  "new_root": "0x1",
  "old_root": "0x2",
  "state_diff": {
    "storage_diffs": {
      "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": [
        {
          "key": "0x5",
          "value": "0x22b"
        },
        {
          "key": "0x2",
          "value": "0x1"
        }
      ],
      "0x3": [
        {
          "key": "0x1",
          "value": "0x5"
        },
        {
          "key": "0x7",
          "value": "0x13"
        }
      ]
    },
    "nonces": { 
		"0x37" : "0x44",
		"0x44" : "0x37"
	},
    "deployed_contracts": [
      {
        "address": "0x1",
        "class_hash": "0x2"
      },
      {
        "address": "0x3",
        "class_hash": "0x4"
      }
	],
    "declared_contracts": [
		"0x37", "0x44"
	]
  }
}`)

	var gatewayStateUpdate clients.StateUpdate
	err := json.Unmarshal(jsonData, &gatewayStateUpdate)
	assert.Equal(t, nil, err, "Unexpected error")

	coreStateUpdate, err := AdaptStateUpdate(&gatewayStateUpdate)
	if assert.NoError(t, err) {
		assert.Equal(t, true, gatewayStateUpdate.NewRoot.Equal(coreStateUpdate.NewRoot))
		assert.Equal(t, true, gatewayStateUpdate.OldRoot.Equal(coreStateUpdate.OldRoot))
		assert.Equal(t, true, gatewayStateUpdate.BlockHash.Equal(coreStateUpdate.BlockHash))

		assert.Equal(t, 2, len(gatewayStateUpdate.StateDiff.DeclaredContracts))
		for idx := range gatewayStateUpdate.StateDiff.DeclaredContracts {
			gw := gatewayStateUpdate.StateDiff.DeclaredContracts[idx]
			core := coreStateUpdate.StateDiff.DeclaredContracts[idx]
			assert.Equal(t, true, gw.Equal(core))
		}

		for keyStr, gw := range gatewayStateUpdate.StateDiff.Nonces {
			key, _ := new(felt.Felt).SetString(keyStr)
			core := coreStateUpdate.StateDiff.Nonces[*key]
			assert.Equal(t, true, gw.Equal(core))
		}

		assert.Equal(t, 2, len(gatewayStateUpdate.StateDiff.DeployedContracts))
		for idx := range gatewayStateUpdate.StateDiff.DeployedContracts {
			gw := gatewayStateUpdate.StateDiff.DeployedContracts[idx]
			core := coreStateUpdate.StateDiff.DeployedContracts[idx]
			assert.Equal(t, true, gw.ClassHash.Equal(core.ClassHash))
			assert.Equal(t, true, gw.Address.Equal(core.Address))
		}

		assert.Equal(t, 2, len(gatewayStateUpdate.StateDiff.StorageDiffs))
		for keyStr, diffs := range gatewayStateUpdate.StateDiff.StorageDiffs {
			key, _ := new(felt.Felt).SetString(keyStr)
			coreDiffs := coreStateUpdate.StateDiff.StorageDiffs[*key]
			assert.Equal(t, len(diffs) > 0, true)
			assert.Equal(t, len(diffs), len(coreDiffs))
			for idx := range diffs {
				assert.Equal(t, true, diffs[idx].Key.Equal(coreDiffs[idx].Key))
				assert.Equal(t, true, diffs[idx].Value.Equal(coreDiffs[idx].Value))
			}
		}
	}
}

func TestAdaptClass(t *testing.T) {
	response := new(clients.ClassDefinition)
	err := json.Unmarshal(classJson, response)
	assert.NoError(t, err)

	class, err := adaptClass(response)
	assert.NoError(t, err)

	assert.Equal(t, new(felt.Felt).SetUint64(0), class.APIVersion)

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
		assert.Equal(t, new(felt.Felt).SetBytes([]byte(v)), class.Bytecode[i])
	}
	assert.Equal(t, len(response.Program.Data), len(class.Bytecode))

	programHash, err := clients.ProgramHash(response)
	assert.NoError(t, err)
	assert.Equal(t, programHash, class.ProgramHash)
}

func TestAdaptInvokeTransaction(t *testing.T) {
	response := new(clients.TransactionStatus)
	err := json.Unmarshal(invokeJson, response)
	assert.NoError(t, err)

	transaction := response.Transaction
	invokeTx := adaptInvokeTransaction(transaction)
	assert.Equal(t, transaction.ContractAddress, invokeTx.ContractAddress)
	assert.Equal(t, transaction.EntryPointSelector, invokeTx.EntryPointSelector)
	assert.Equal(t, transaction.SenderAddress, invokeTx.SenderAddress)
	assert.Equal(t, transaction.Nonce, invokeTx.Nonce)
	assert.Equal(t, transaction.Calldata, invokeTx.CallData)
	assert.Equal(t, transaction.Signature, invokeTx.Signature)
	assert.Equal(t, transaction.MaxFee, invokeTx.MaxFee)
	assert.Equal(t, transaction.Version, invokeTx.Version)
}

func TestAdaptDeployTransaction(t *testing.T) {
	response := new(clients.TransactionStatus)
	err := json.Unmarshal(deployJson, response)
	assert.NoError(t, err)

	transaction := response.Transaction
	deployTx, err := adaptDeployTransaction(transaction)
	assert.NoError(t, err)

	assert.Equal(t, transaction.ContractAddressSalt, deployTx.ContractAddressSalt)
	assert.Equal(t, transaction.ConstructorCalldata, deployTx.ConstructorCallData)
	assert.Equal(t, transaction.ContractAddress, deployTx.CallerAddress)
	assert.Equal(t, transaction.Version, deployTx.Version)
	assert.Equal(t, transaction.ClassHash, deployTx.ClassHash)
}

func TestAdaptDeclareTransaction(t *testing.T) {
	response := new(clients.TransactionStatus)
	err := json.Unmarshal(declareJson, response)
	assert.NoError(t, err)

	transaction := response.Transaction
	declareTx, err := adaptDeclareTransaction(transaction)
	assert.NoError(t, err)

	assert.Equal(t, transaction.SenderAddress, declareTx.SenderAddress)
	assert.Equal(t, transaction.Version, declareTx.Version)
	assert.Equal(t, transaction.Nonce, declareTx.Nonce)
	assert.Equal(t, transaction.MaxFee, declareTx.MaxFee)
	assert.Equal(t, transaction.Signature, declareTx.Signature)
	assert.Equal(t, transaction.ClassHash, declareTx.ClassHash)
}
