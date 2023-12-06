package builder

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/validator"
)

type GenesisConfig struct {
	ChainID string `json:"chain_id" validate:"required"`

	Classes []string `json:"classes"` // []path-to-class.json

	Contracts map[string]GenesisContractData `json:"contracts"` // address -> {classHash, constructorArgs}

	FunctionCalls []FunctionCall `json:"function_calls"` // list of function calls to execute
}

type GenesisContractData struct {
	ClassHash       felt.Felt   `json:"class_hash"`
	ConstructorArgs []felt.Felt `json:"constructor_args"`
}
type FunctionCall struct {
	ContractAddress    felt.Felt   `json:"contract_address"`
	EntryPointSelector felt.Felt   `json:"entry_point_selector"`
	Calldata           []felt.Felt `json:"calldata"`
}

func (g *GenesisConfig) Validate() error {
	validate := validator.Validator()
	return validate.Struct(g)
}
