package rpcv9

import (
	"encoding/json"
	"errors"

	hintRunnerZero "github.com/NethermindEth/cairo-vm-go/pkg/hintrunner/zero"
	"github.com/NethermindEth/cairo-vm-go/pkg/parsers/zero"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
)

type EntryPoint struct {
	Offset   uint64    `json:"offset"`
	Selector felt.Felt `json:"selector"`
	Builtins []string  `json:"builtins"`
}

type EntryPointsByType struct {
	Constructor []EntryPoint `json:"CONSTRUCTOR"`
	External    []EntryPoint `json:"EXTERNAL"`
	L1Handler   []EntryPoint `json:"L1_HANDLER"`
}

// CompiledCasmResponse represents a compiled Cairo class. It follows this specification:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_executables.json#L45
type CompiledCasmResponse struct {
	EntryPointsByType EntryPointsByType `json:"entry_points_by_type"`
	// can't use felt.Felt here because prime is larger than felt
	Prime                  string          `json:"prime"`
	CompilerVersion        string          `json:"compiler_version"`
	Bytecode               []*felt.Felt    `json:"bytecode"`
	Hints                  json.RawMessage `json:"hints"`
	BytecodeSegmentLengths []int           `json:"bytecode_segment_lengths,omitempty"`
}

// CompiledCasm receives a class hash and returns the compiled cairo assembly (CASM)
func (h *Handler) CompiledCasm(classHash *felt.Felt) (CompiledCasmResponse, *jsonrpc.Error) {
	state, stateCloser, err := h.bcReader.HeadState()
	if err != nil {
		return CompiledCasmResponse{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(stateCloser, "failed to close state reader")

	declaredClass, err := state.Class(classHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return CompiledCasmResponse{}, rpccore.ErrClassHashNotFound
		}
		return CompiledCasmResponse{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch class := declaredClass.Class.(type) {
	case *core.DeprecatedCairoClass:
		resp, err := adaptDeprecatedCairoClass(class)
		if err != nil {
			return CompiledCasmResponse{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
		return resp, nil
	case *core.SierraClass:
		return adaptCasmClass(class.Compiled), nil
	}

	return CompiledCasmResponse{}, jsonrpc.Err(jsonrpc.InternalError, "unsupported class type")
}

func adaptDeprecatedCairoClass(class *core.DeprecatedCairoClass) (CompiledCasmResponse, error) {
	program, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return CompiledCasmResponse{}, err
	}

	var deprecatedCairo zero.ZeroProgram
	err = json.Unmarshal(program, &deprecatedCairo)
	if err != nil {
		return CompiledCasmResponse{}, err
	}

	bytecode := make([]*felt.Felt, len(deprecatedCairo.Data))
	for i, str := range deprecatedCairo.Data {
		f, err := felt.NewFromString[felt.Felt](str)
		if err != nil {
			return CompiledCasmResponse{}, err
		}
		bytecode[i] = f
	}

	classHints, err := hintRunnerZero.GetZeroHints(&deprecatedCairo)
	if err != nil {
		return CompiledCasmResponse{}, err
	}

	// slice of 2-element tuples where first value is pc, and second value is slice of hints
	hints := make([][2]any, len(classHints))
	var count int
	for pc, hintItems := range utils.SortedMap(classHints) {
		hints[count] = [2]any{pc, hintItems}
		count += 1
	}
	rawHints, err := json.Marshal(hints)
	if err != nil {
		return CompiledCasmResponse{}, err
	}

	result := CompiledCasmResponse{
		EntryPointsByType: EntryPointsByType{
			Constructor: adaptDeprecatedEntryPoints(class.Constructors),
			External:    adaptDeprecatedEntryPoints(class.Externals),
			L1Handler:   adaptDeprecatedEntryPoints(class.L1Handlers),
		},
		Prime:                  deprecatedCairo.Prime,
		Bytecode:               bytecode,
		CompilerVersion:        deprecatedCairo.CompilerVersion,
		Hints:                  json.RawMessage(rawHints),
		BytecodeSegmentLengths: nil, // Cairo 0 classes don't have this field (it was introduced since Sierra 1.5.0)
	}
	return result, nil
}

func adaptDeprecatedEntryPoints(deprecatedEntryPoints []core.DeprecatedEntryPoint) []EntryPoint {
	entryPoints := make([]EntryPoint, len(deprecatedEntryPoints))
	for i := range deprecatedEntryPoints {
		entryPoints[i] = EntryPoint{
			Offset:   deprecatedEntryPoints[i].Offset.Uint64(),
			Selector: *deprecatedEntryPoints[i].Selector,
			Builtins: nil,
		}
	}
	return entryPoints
}

func adaptCasmClass(class *core.CasmClass) CompiledCasmResponse {
	result := CompiledCasmResponse{
		EntryPointsByType: EntryPointsByType{
			Constructor: adaptCasmEntryPoints(class.Constructor),
			External:    adaptCasmEntryPoints(class.External),
			L1Handler:   adaptCasmEntryPoints(class.L1Handler),
		},
		Prime:                  utils.ToHex(class.Prime),
		CompilerVersion:        class.CompilerVersion,
		Bytecode:               class.Bytecode,
		Hints:                  class.Hints,
		BytecodeSegmentLengths: collectSegmentLengths(class.BytecodeSegmentLengths),
	}
	return result
}

func adaptCasmEntryPoints(casmEntryPoints []core.CasmEntryPoint) []EntryPoint {
	entryPoints := make([]EntryPoint, len(casmEntryPoints))
	for i := range casmEntryPoints {
		entryPoints[i] = EntryPoint{
			Offset:   casmEntryPoints[i].Offset,
			Selector: *casmEntryPoints[i].Selector,
			Builtins: casmEntryPoints[i].Builtins,
		}
	}
	return entryPoints
}

func collectSegmentLengths(segmentLengths core.SegmentLengths) []int {
	if len(segmentLengths.Children) == 0 {
		return []int{int(segmentLengths.Length)}
	}

	var result []int
	for _, child := range segmentLengths.Children {
		result = append(result, collectSegmentLengths(child)...)
	}

	return result
}
