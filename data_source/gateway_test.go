package datasource

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

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

	coreStateUpdate, err := adaptStateUpdate(&gatewayStateUpdate)
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
	classJson, err := os.ReadFile("testdata/class.json")
	if err != nil {
		t.Error(err)
	}

	response := new(clients.ClassDefinition)
	err = json.Unmarshal(classJson, response)
	if err != nil {
		t.Error(err)
	}

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
		assert.Equal(t, v, class.Bytecode[i])
	}
	assert.Equal(t, len(response.Program.Data), len(class.Bytecode))

	// TODO:
	// programHash :=
	// assert.Equal(t, programHash, class.ProgramHash)

}
