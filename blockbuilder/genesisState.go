package blockbuilder

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/validator"
)

type GenesisConfig struct {
	ChainID                 string           `json:"chain_id" validate:"required"`
	WhitelistedSequencerSet []felt.Felt      `json:"whitelisted_sequencer_set"`
	State                   genesisStateData `json:"state"`
}

type genesisStateData struct {
	Classes       []string                          `json:"classes"` // []path-to-class.json
	Contracts     map[felt.Felt]genesisContractData `json:"contracts"`
	FunctionCalls []functionCall                    `json:"function_calls"`
}

type genesisContractData struct {
	ClassHash       felt.Felt   `json:"class_hash"`
	ConstructorArgs []felt.Felt `json:"constructor_args"`
}
type functionCall struct {
	ContractAddress    felt.Felt   `json:"contract_address"`
	EntryPointSelector felt.Felt   `json:"entry_point_selector"`
	Calldata           []felt.Felt `json:"calldata"`
}

func (g *GenesisConfig) Validate() error {
	validator := validator.Validator()
	return validator.Struct(g)
}
