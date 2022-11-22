package clients

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestStateUpdateUnmarshal(t *testing.T) {
	jsonData := []byte(`{
  "block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
  "new_root": "021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6",
  "old_root": "0000000000000000000000000000000000000000000000000000000000000000",
  "state_diff": {
    "storage_diffs": {
      "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": [
        {
          "key": "0x5",
          "value": "0x22b"
        }
      ]
    },
    "nonces": {},
    "deployed_contracts": [
      {
        "address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
        "class_hash": "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"
      }
	],
    "declared_contracts": []
  }
}`)

	var update StateUpdate
	err := json.Unmarshal(jsonData, &update)
	assert.Equal(t, nil, err, "Unexpected error")
	expected, _ := new(felt.Felt).SetString("0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943")
	assert.Equal(t, true, update.BlockHash.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6")
	assert.Equal(t, true, update.NewRoot.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x0000000000000000000000000000000000000000000000000000000000000000")
	assert.Equal(t, true, update.OldRoot.Equal(expected))
	assert.Equal(t, 1, len(update.StateDiff.StorageDiffs))

	diffs, found := update.StateDiff.StorageDiffs["0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6"]
	assert.Equal(t, true, found)
	assert.Equal(t, 1, len(diffs))

	expected, _ = new(felt.Felt).SetString("0x5")
	assert.Equal(t, true, diffs[0].Key.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x22b")
	assert.Equal(t, true, diffs[0].Value.Equal(expected))

	assert.Equal(t, 1, len(update.StateDiff.DeployedContracts))
	expected, _ = new(felt.Felt).SetString("0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
	assert.Equal(t, true, update.StateDiff.DeployedContracts[0].Address.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	assert.Equal(t, true, update.StateDiff.DeployedContracts[0].ClassHash.Equal(expected))
}
