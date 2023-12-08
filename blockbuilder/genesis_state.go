package blockbuilder

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/validator"
)

type GenesisConfig struct {
	ChainID string `json:"chain_id" validate:"required"`
	// Classes: Paths to 'class.json' files defining classes for the genesis block.
	Classes []string `json:"classes"`

	// Contracts: Mapping of contract addresses to their initialisation data.
	// Class constructors called with "ConstructorArgs" will be deployed in the
	// genesis block. The contract address needs to match what the class generates.
	Contracts map[string]GenesisContractData `json:"contracts"`

	// FunctionCalls: List of function calls whose resulting state
	// changes are applied to the genesis state. E.g. calling mint(amount,address)
	// on a token contract will result in the address having "amount" tokens.
	FunctionCalls []FunctionCall `json:"function_calls"`
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
