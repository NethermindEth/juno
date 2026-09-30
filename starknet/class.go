package starknet

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
)

// EntryPointOffset accepts both decimal integers and hex strings in JSON,
// since the feeder gateway has used both formats across different class versions.
type EntryPointOffset felt.Felt

func (o *EntryPointOffset) UnmarshalJSON(data []byte) error {
	const maxLen = fp.Bits * 3
	if len(data) > maxLen {
		return fmt.Errorf("value too large: got %d bytes, max is %d", len(data), maxLen)
	}

	s := strings.Trim(string(data), `"`)
	_, err := (*felt.Felt)(o).SetString(s)
	return err
}

func (o EntryPointOffset) MarshalText() ([]byte, error) {
	return felt.Felt(o).MarshalText()
}

func (o EntryPointOffset) AppendText(data []byte) ([]byte, error) {
	return felt.Felt(o).AppendText(data)
}

func (o EntryPointOffset) String() string {
	return (*felt.Felt)(&o).String()
}

type EntryPoint struct {
	Selector *felt.Felt        `json:"selector"`
	Offset   *EntryPointOffset `json:"offset"`
}

type SierraEntryPoints struct {
	Constructor []SierraEntryPoint `json:"CONSTRUCTOR"`
	External    []SierraEntryPoint `json:"EXTERNAL"`
	L1Handler   []SierraEntryPoint `json:"L1_HANDLER"`
}

type SierraClass struct {
	Abi         string            `json:"abi,omitempty"`
	EntryPoints SierraEntryPoints `json:"entry_points_by_type"`
	Program     []felt.Felt       `json:"sierra_program"`
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

type DeprecatedCairoClass struct {
	Abi         json.RawMessage `json:"abi"`
	EntryPoints EntryPoints     `json:"entry_points_by_type"`
	Program     json.RawMessage `json:"program"`
}

type ClassDefinition struct {
	DeprecatedCairo *DeprecatedCairoClass
	Sierra          *SierraClass
}

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.
func (c *ClassDefinition) Validate() error {
	return nil
}

// UnmarshalJSONFrom decodes a Sierra class when the object has a sierra_program key,
// else a deprecated Cairo class.
func (c *ClassDefinition) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	value, err := dec.ReadValue()
	if err != nil {
		return err
	}

	isSierra, err := hasTopLevelKey(value, "sierra_program", dec.Options())
	if err != nil {
		return err
	}
	if isSierra {
		c.Sierra = new(SierraClass)
		return jsonv2.Unmarshal(value, c.Sierra, dec.Options())
	}
	c.DeprecatedCairo = new(DeprecatedCairoClass)
	return jsonv2.Unmarshal(value, c.DeprecatedCairo, dec.Options())
}

// hasTopLevelKey scans the keys of a JSON object without decoding its values.
// A value that is not an object has no keys.
func hasTopLevelKey(value jsontext.Value, key string, opts jsontext.Options) (bool, error) {
	dec := jsontext.NewDecoder(bytes.NewBuffer(value), opts)
	open, err := dec.ReadToken()
	if err != nil {
		return false, err
	}
	if open.Kind() != '{' {
		return false, nil
	}
	for dec.PeekKind() == '"' {
		name, err := dec.ReadToken()
		if err != nil {
			return false, err
		}
		if name.String() == key {
			return true, nil
		}
		if err := dec.SkipValue(); err != nil {
			return false, err
		}
	}
	return false, nil
}

// SegmentLengths is a CASM bytecode segment: a leaf length or a list of child segments.
type SegmentLengths struct {
	Children []SegmentLengths
	Length   uint64
}

func (n *SegmentLengths) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() == '[' {
		return jsonv2.UnmarshalDecode(dec, &n.Children)
	}
	return jsonv2.UnmarshalDecode(dec, &n.Length)
}

func (n SegmentLengths) MarshalJSONTo(enc *jsontext.Encoder) error {
	if len(n.Children) > 0 {
		return jsonv2.MarshalEncode(enc, n.Children)
	}
	return enc.WriteToken(jsontext.Uint(n.Length))
}

type CasmClass struct {
	Prime                  string          `json:"prime"`
	Bytecode               []felt.Felt     `json:"bytecode"`
	Hints                  json.RawMessage `json:"hints"`
	PythonicHints          json.RawMessage `json:"pythonic_hints"`
	CompilerVersion        string          `json:"compiler_version"`
	BytecodeSegmentLengths *SegmentLengths `json:"bytecode_segment_lengths,omitempty"`
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

// IsDeprecatedCompiledClassDefinition reports whether a compiled class is Cairo 0,
// which has a program key.
func IsDeprecatedCompiledClassDefinition(definition json.RawMessage) (bool, error) {
	return hasTopLevelKey(definition, "program", json.DefaultOptionsV1())
}
