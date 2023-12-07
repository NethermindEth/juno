package blockbuilder

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

var ErrChainIDRequired = errors.New("ChainID is required")

type GenesisConfig struct {
	ChainID                 string           `json:"chain_id"`
	WhitelistedSequencerSet []felt.Felt      `json:"whitelisted_sequencer_set"`
	State                   genesisStateData `json:"state"`
}

type genesisStateData struct {
	Classes   map[string]genesisClassData    `json:"classes"`
	Contracts map[string]genesisContractData `json:"contracts"`
	Storage   map[string][]core.StorageDiff  `json:"storage"`
}

type genesisClassData struct {
	Path    string `json:"path"`
	Version int    `json:"version"`
}

type genesisContractData struct {
	ClassHash       felt.Felt    `json:"class_hash"`
	ConstructorArgs *[]felt.Felt `json:"constructor_args"`
}

func (g *GenesisConfig) UnmarshalJSON(data []byte) error {
	type genesisConfig GenesisConfig
	if err := json.Unmarshal(data, (*genesisConfig)(g)); err != nil {
		return err
	}
	if g.ChainID == "" {
		return ErrChainIDRequired
	}
	return nil
}
