package types

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/felt"
)

type Abi []struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Inputs []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"inputs"`
	Outputs []interface{} `json:"outputs"`
}

type Contract struct {
	Abi      Abi
	Bytecode []*felt.Felt

	FullDef json.RawMessage
}

// UnmarshalJSON unmarshals the JSON-encoded data into the contract.
func (c *Contract) UnmarshalJSON(data []byte) error {
	var fullDef json.RawMessage
	if err := json.Unmarshal(data, &fullDef); err != nil {
		return err
	}

	var contract struct {
		Abi     Abi `json:"abi"`
		Program struct {
			Data []*felt.Felt `json:"data"`
		} `json:"program"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		return err
	}

	c.Abi = contract.Abi
	c.Bytecode = contract.Program.Data
	c.FullDef = fullDef
	return nil
}

// MarshalJSON marshals the contract into JSON-encoded data.
func (c *Contract) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.FullDef)
}
