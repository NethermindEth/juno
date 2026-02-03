package rpcv8

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

type CasmEntryPoint struct {
	Offset   uint64     `json:"offset"`
	Selector *felt.Felt `json:"selector"`
	Builtins []string   `json:"builtins"`
}

type EntryPointsByType struct {
	Constructor []CasmEntryPoint `json:"CONSTRUCTOR"`
	External    []CasmEntryPoint `json:"EXTERNAL"`
	L1Handler   []CasmEntryPoint `json:"L1_HANDLER"`
}

// CasmCompiledContractClass follows this specification:
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.0-rc0/api/starknet_executables.json#L45
type CasmCompiledContractClass struct {
	EntryPointsByType EntryPointsByType `json:"entry_points_by_type"`
	// can't use felt.Felt here because prime is larger than felt
	Prime                  string          `json:"prime"`
	CompilerVersion        string          `json:"compiler_version"`
	Bytecode               []*felt.Felt    `json:"bytecode"`
	Hints                  json.RawMessage `json:"hints"`
	BytecodeSegmentLengths []int           `json:"bytecode_segment_lengths,omitempty"`
}

func (h *Handler) CompiledCasm(
	classHash *felt.Felt,
) (CasmCompiledContractClass, *jsonrpc.Error) {
	state, stateCloser, err := h.bcReader.HeadState()
	if err != nil {
		return CasmCompiledContractClass{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(stateCloser, "failed to close state reader")

	declaredClass, err := state.Class(classHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return CasmCompiledContractClass{}, rpccore.ErrClassHashNotFound
		}
		return CasmCompiledContractClass{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch class := declaredClass.Class.(type) {
	case *core.DeprecatedCairoClass:
		return CasmCompiledContractClass{}, rpccore.ErrClassHashNotFound
	case *core.SierraClass:
		return adaptCasmClass(class.Compiled), nil
	}

	return CasmCompiledContractClass{}, jsonrpc.Err(jsonrpc.InternalError, "unsupported class type")
}

func adaptCasmClass(class *core.CasmClass) CasmCompiledContractClass {
	adaptEntryPoint := func(ep core.CasmEntryPoint) CasmEntryPoint {
		return CasmEntryPoint{
			Offset:   ep.Offset,
			Selector: ep.Selector,
			Builtins: ep.Builtins,
		}
	}

	return CasmCompiledContractClass{
		EntryPointsByType: EntryPointsByType{
			Constructor: utils.Map(class.Constructor, adaptEntryPoint),
			External:    utils.Map(class.External, adaptEntryPoint),
			L1Handler:   utils.Map(class.L1Handler, adaptEntryPoint),
		},
		Prime:                  utils.ToHex(class.Prime),
		CompilerVersion:        class.CompilerVersion,
		Bytecode:               class.Bytecode,
		Hints:                  class.Hints,
		BytecodeSegmentLengths: collectSegmentLengths(class.BytecodeSegmentLengths),
	}
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
