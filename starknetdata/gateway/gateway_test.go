package gateway

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

var (
	//go:embed testdata/block_11817.json
	blockJson []byte
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
	err := json.Unmarshal(blockJson, &response)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %s", err)
	}

	block, err := AdaptBlock(response)
	if !assert.NoError(t, err) {
		t.Errorf("unexpected error on AdaptBlock: %s", err)
	}
	assert.True(t, block.Hash.Equal(response.Hash))
	assert.True(t, block.ParentHash.Equal(response.ParentHash))
	assert.Equal(t, block.Number, response.Number)
	assert.True(t, block.GlobalStateRoot.Equal(response.StateRoot))
	assert.True(t, block.Timestamp.Equal(new(felt.Felt).SetUint64(response.Timestamp)))
	assert.Equal(t, block.TransactionCount, new(felt.Felt).SetUint64(uint64(len(response.Transactions))))
	assert.Equal(t, block.ProtocolVersion, new(felt.Felt))
	var extraData *felt.Felt
	assert.Equal(t, block.ExtraData, extraData)
	// TODO test transaction commitment...?
	// TODO test event commitment and count
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

	invokeTx := adaptInvokeTransaction(response)
	assert.Equal(t, response.Transaction.ContractAddress, invokeTx.ContractAddress)
	assert.Equal(t, response.Transaction.EntryPointSelector, invokeTx.EntryPointSelector)
	assert.Equal(t, response.Transaction.SenderAddress, invokeTx.SenderAddress)
	assert.Equal(t, response.Transaction.Nonce, invokeTx.Nonce)
	assert.Equal(t, response.Transaction.Calldata, invokeTx.CallData)
	assert.Equal(t, response.Transaction.Signature, invokeTx.Signature)
	assert.Equal(t, response.Transaction.MaxFee, invokeTx.MaxFee)
	assert.Equal(t, response.Transaction.Version, invokeTx.Version)
}

func getMockClass() (*clients.ClassDefinition, *core.Class) {
	response := new(clients.ClassDefinition)
	json.Unmarshal(classJson, response)
	class, _ := adaptClass(response)

	return response, class
}

func TestAdaptDeployTransaction(t *testing.T) {
	response := new(clients.TransactionStatus)
	err := json.Unmarshal(deployJson, response)
	assert.NoError(t, err)

	deployTx, err := adaptDeployTransaction(response)
	assert.NoError(t, err)

	assert.Equal(t, response.Transaction.ContractAddressSalt, deployTx.ContractAddressSalt)
	assert.Equal(t, response.Transaction.ConstructorCalldata, deployTx.ConstructorCallData)
	assert.Equal(t, response.Transaction.ContractAddress, deployTx.CallerAddress)
	assert.Equal(t, response.Transaction.Version, deployTx.Version)
	assert.Equal(t, response.Transaction.ClassHash, deployTx.ClassHash)
}

func TestAdaptDeclareTransaction(t *testing.T) {
	response := new(clients.TransactionStatus)
	err := json.Unmarshal(declareJson, response)
	assert.NoError(t, err)

	declareTx, err := adaptDeclareTransaction(response)
	assert.NoError(t, err)

	assert.Equal(t, response.Transaction.SenderAddress, declareTx.SenderAddress)
	assert.Equal(t, response.Transaction.Version, declareTx.Version)
	assert.Equal(t, response.Transaction.Nonce, declareTx.Nonce)
	assert.Equal(t, response.Transaction.MaxFee, declareTx.MaxFee)
	assert.Equal(t, response.Transaction.Signature, declareTx.Signature)
	assert.Equal(t, response.Transaction.ClassHash, declareTx.ClassHash)
}

func testTransactionClass(t *testing.T, expected *clients.ClassDefinition, actual *core.Class) {
	assert.Equal(t, new(felt.Felt).SetUint64(0), actual.APIVersion)

	for i, v := range expected.EntryPoints.External {
		assert.Equal(t, v.Selector, actual.Externals[i].Selector)
		assert.Equal(t, v.Offset, actual.Externals[i].Offset)
	}
	assert.Equal(t, len(expected.EntryPoints.External), len(actual.Externals))

	for i, v := range expected.EntryPoints.L1Handler {
		assert.Equal(t, v.Selector, actual.L1Handlers[i].Selector)
		assert.Equal(t, v.Offset, actual.L1Handlers[i].Offset)
	}
	assert.Equal(t, len(expected.EntryPoints.L1Handler), len(actual.L1Handlers))

	for i, v := range expected.EntryPoints.Constructor {
		assert.Equal(t, v.Selector, actual.Constructors[i].Selector)
		assert.Equal(t, v.Offset, actual.Constructors[i].Offset)
	}
	assert.Equal(t, len(expected.EntryPoints.Constructor), len(actual.Constructors))

	for i, v := range expected.Program.Builtins {
		assert.Equal(t, new(felt.Felt).SetBytes([]byte(v)), actual.Builtins[i])
	}
	assert.Equal(t, len(expected.Program.Builtins), len(actual.Builtins))

	for i, v := range expected.Program.Data {
		assert.Equal(t, new(felt.Felt).SetBytes([]byte(v)), actual.Bytecode[i])
	}
	assert.Equal(t, len(expected.Program.Data), len(actual.Bytecode))
}
