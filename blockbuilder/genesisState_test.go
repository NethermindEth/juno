package blockbuilder_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/blockbuilder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestGenesisUnmarshal(t *testing.T) {

	genesisJSONStr := `{
		"chain_id": "SN_GOERLI",
		"whitelisted_sequencer_set": [
			"0x1",
			"0x2"
		],
		"state": {
			"classes": {
				"0x3": {
					"path": "./config/account.json",
					"version": 0
				}
			},
			"contracts": {
				"0x4": {
					"class_hash": "0x123",
					"constructor_args": []
				  }
			},
			"storage": {
				"0x5": [
					{
						"key": "0x0341c1bdfd89f69748aa00b5742b03adbffd79b8e80cab5c50d91cd8c2a79be1",
						"value": "0x4574686572"
					}
				]
			}
		}
	}
	`

	var genesis blockbuilder.GenesisConfig
	require.NoError(t, json.Unmarshal([]byte(genesisJSONStr), &genesis))

	t.Run("test chain ID", func(t *testing.T) {
		require.Equal(t, "SN_GOERLI", genesis.ChainID)
	})

	t.Run("test whitelisted sequencer set", func(t *testing.T) {
		require.Equal(t, 2, len(genesis.WhitelistedSequencerSet))
		require.Equal(t, new(felt.Felt).SetUint64(1).String(), genesis.WhitelistedSequencerSet[0].String())
	})

	t.Run("test classes", func(t *testing.T) {
		for classHash, class := range genesis.State.Classes {
			require.Equal(t, classHash, "0x3")
			require.Equal(t, class.Path, "./config/account.json")
			require.Equal(t, class.Version, 0)
		}
	})

	t.Run("test contracts", func(t *testing.T) {
		for contractHash, contract := range genesis.State.Contracts {
			require.Equal(t, contractHash, "0x4")
			require.Equal(t, contract.ClassHash.String(), "0x123")
			require.Equal(t, contract.ConstructorArgs, &[]felt.Felt{})
		}
	})

	t.Run("test storage", func(t *testing.T) {
		for contractAddress, storage := range genesis.State.Storage {
			require.Equal(t, contractAddress, "0x5")
			require.Equal(t, storage[0].Key.String(), "0x341c1bdfd89f69748aa00b5742b03adbffd79b8e80cab5c50d91cd8c2a79be1")
			require.Equal(t, storage[0].Value.String(), "0x4574686572")
		}
	})

	t.Run("test unmarshal validation", func(t *testing.T) {
		var genesisFalse blockbuilder.GenesisConfig
		require.ErrorIs(t, json.Unmarshal([]byte("{}"), &genesisFalse), blockbuilder.ErrChainIDRequired)
	})

}
