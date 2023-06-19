package feeder

import (
	"encoding/json"

	"github.com/NethermindEth/juno/core/felt"
)

type EntryPoint struct {
	Selector *felt.Felt `json:"selector"`
	Offset   *felt.Felt `json:"offset"`
}

type SierraDefinition struct {
	Abi         string `json:"abi"`
	EntryPoints struct {
		Constructor []SierraEntryPoint `json:"CONSTRUCTOR"`
		External    []SierraEntryPoint `json:"EXTERNAL"`
		L1Handler   []SierraEntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program []*felt.Felt `json:"sierra_program"`
	Version string       `json:"contract_class_version"`
}

type SierraEntryPoint struct {
	Index    uint64     `json:"function_idx"`
	Selector *felt.Felt `json:"selector"`
}

type Cairo0Definition struct {
	Abi         json.RawMessage `json:"abi"`
	EntryPoints struct {
		Constructor []EntryPoint `json:"CONSTRUCTOR"`
		External    []EntryPoint `json:"EXTERNAL"`
		L1Handler   []EntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program json.RawMessage `json:"program"`
}

type ClassDefinition struct {
	V0 *Cairo0Definition
	V1 *SierraDefinition
}

func (c *ClassDefinition) UnmarshalJSON(data []byte) error {
	jsonMap := make(map[string]any)
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		return err
	}

	if _, found := jsonMap["sierra_program"]; found {
		c.V1 = new(SierraDefinition)
		return json.Unmarshal(data, c.V1)
	}
	c.V0 = new(Cairo0Definition)
	return json.Unmarshal(data, c.V0)
}
