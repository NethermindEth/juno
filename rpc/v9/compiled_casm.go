package rpcv9

import (
	"encoding/json"
	"errors"

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
		return CompiledCasmResponse{}, rpccore.ErrClassHashNotFound
	case *core.SierraClass:
		return adaptCasmClass(class.Compiled), nil
	}

	return CompiledCasmResponse{}, jsonrpc.Err(jsonrpc.InternalError, "unsupported class type")
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
