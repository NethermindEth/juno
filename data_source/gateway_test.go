package dataSource

import (
	"encoding/json"
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
