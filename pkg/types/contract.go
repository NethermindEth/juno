package types

import "encoding/json"

type Abi []struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Inputs []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"inputs"`
	Outputs []interface{} `json:"outputs"`
}

type ContractDefinition struct {
	Program struct {
		DebugInfo        string   `json:"debug_info"`
		Attributes       []string `json:"attributes"`
		ReferenceManager string   `json:"reference_manager"`
		Builtins         []string `json:"builtins"`
		Bytecode         []*Felt  `json:"data"`
		Prime            string   `json:"prime"`
		Hints            []string `json:"hints"`
		MainScope        string   `json:"main_scope"`
		Identifiers      string   `json:"identifiers"`
	} `json:"program"`
	EntryPointsByType struct {
		External []struct {
			Offset   string `json:"offset"`
			Selector string `json:"selector"`
		} `json:"EXTERNAL"`
		L1Handler   []interface{} `json:"L1_HANDLER"`
		Constructor []interface{} `json:"CONSTRUCTOR"`
	} `json:"entry_points_by_type"`
}

type Contract struct {
	Abi      Abi
	Bytecode []*Felt

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
			Data []*Felt `json:"data"`
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
