package starknet

import (
	"encoding/json"
	"strconv"

	"github.com/NethermindEth/juno/core/felt"
)

type EntryPoint struct {
	Selector *felt.Felt `json:"selector"`
	Offset   *felt.Felt `json:"offset"`
}

type SierraEntryPoints struct {
	Constructor []SierraEntryPoint `json:"CONSTRUCTOR"`
	External    []SierraEntryPoint `json:"EXTERNAL"`
	L1Handler   []SierraEntryPoint `json:"L1_HANDLER"`
}

type SierraDefinition struct {
	Abi         string            `json:"abi,omitempty"`
	EntryPoints SierraEntryPoints `json:"entry_points_by_type"`
	Program     []*felt.Felt      `json:"sierra_program"`
	Version     string            `json:"contract_class_version"`
}

type SierraEntryPoint struct {
	Index    uint64     `json:"function_idx"`
	Selector *felt.Felt `json:"selector"`
}

type EntryPoints struct {
	Constructor []EntryPoint `json:"CONSTRUCTOR"`
	External    []EntryPoint `json:"EXTERNAL"`
	L1Handler   []EntryPoint `json:"L1_HANDLER"`
}

type Cairo0Definition struct {
	Abi         json.RawMessage `json:"abi"`
	EntryPoints EntryPoints     `json:"entry_points_by_type"`
	Program     json.RawMessage `json:"program"`
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

type SegmentLengths struct {
	Children []SegmentLengths
	Length   uint64
}

func (n *SegmentLengths) UnmarshalJSON(data []byte) error {
	var err error
	n.Length, err = strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return json.Unmarshal(data, &n.Children)
	}
	return err
}

func (n SegmentLengths) MarshalJSON() ([]byte, error) {
	if len(n.Children) > 0 {
		return json.Marshal(n.Children)
	}
	return json.Marshal(n.Length)
}

type CompiledClass struct {
	Prime                  string          `json:"prime"`
	Bytecode               []*felt.Felt    `json:"bytecode"`
	Hints                  json.RawMessage `json:"hints"`
	PythonicHints          json.RawMessage `json:"pythonic_hints"`
	CompilerVersion        string          `json:"compiler_version"`
	BytecodeSegmentLengths SegmentLengths  `json:"bytecode_segment_lengths"`
	EntryPoints            struct {
		External    []CompiledEntryPoint `json:"EXTERNAL"`
		L1Handler   []CompiledEntryPoint `json:"L1_HANDLER"`
		Constructor []CompiledEntryPoint `json:"CONSTRUCTOR"`
	} `json:"entry_points_by_type"`
}

type CompiledEntryPoint struct {
	Selector *felt.Felt `json:"selector"`
	Offset   uint64     `json:"offset"`
	Builtins []string   `json:"builtins"`
}

func IsDeprecatedCompiledClassDefinition(definition json.RawMessage) (bool, error) {
	var classMap map[string]json.RawMessage
	if err := json.Unmarshal(definition, &classMap); err != nil {
		return false, err
	}
	return len(classMap["program"]) > 0, nil
}
