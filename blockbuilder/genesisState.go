package blockbuilder

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/go-playground/validator/v10"
)

type GenesisConfig struct {
	ChainID                 string           `json:"chain_id" validate:"required"`
	WhitelistedSequencerSet []felt.Felt      `json:"whitelisted_sequencer_set"`
	State                   genesisStateData `json:"state"`
}

type genesisStateData struct {
	Classes       map[felt.Felt]string              `json:"classes"` // map[classHash]path-to-class.json
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
	validate := validator.New()
	return validate.Struct(g)
}
