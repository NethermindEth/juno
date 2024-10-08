package rpc

import (
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
)

type CasmEntryPoint struct {
	Offset   *felt.Felt `json:"offset"`
	Selector *felt.Felt `json:"selector"`
	Builtins []string   `json:"builtins"`
}

type EntryPointsByType struct {
	Constructor []CasmEntryPoint `json:"CONSTRUCTOR"`
	External    []CasmEntryPoint `json:"EXTERNAL"`
	L1Handler   []CasmEntryPoint `json:"L1_HANDLER"`
}

type CasmCompiledContractClass struct {
	EntryPointsByType      EntryPointsByType `json:"entry_points_by_type"`
	Prime                  *felt.Felt        `json:"prime"`
	CompilerVersion        string            `json:"compiler_version"`
	Bytecode               []*felt.Felt      `json:"bytecode"`
	Hints                  json.RawMessage   `json:"hints"`
	BytecodeSegmentLengths []int             `json:"bytecode_segment_lengths,omitempty"`
}

func (h *Handler) CompiledCasm(classHash *felt.Felt) (*CasmCompiledContractClass, *jsonrpc.Error) {
	rd, stateCloser, err := h.bcReader.HeadState()
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(stateCloser, "failed to close state reader")

	declaredClass, err := rd.Class(classHash)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch class := declaredClass.Class.(type) {
	case *core.Cairo0Class:
		program, err := utils.Gzip64Decode(class.Program)
		if err != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
		fmt.Println(string(program))
	case *core.Cairo1Class:
		return adaptCompiledClass(class.Compiled)
	}

	return nil, jsonrpc.Err(jsonrpc.InternalError, "unsupported class type")
}

func adaptCompiledClass(class *core.CompiledClass) (*CasmCompiledContractClass, *jsonrpc.Error) {
	adaptEntryPoint := func(ep core.CompiledEntryPoint) CasmEntryPoint {
		return CasmEntryPoint{
			Offset:   new(felt.Felt).SetUint64(ep.Offset),
			Selector: ep.Selector,
			Builtins: ep.Builtins,
		}
	}

	result := &CasmCompiledContractClass{
		EntryPointsByType: EntryPointsByType{
			Constructor: utils.Map(class.Constructor, adaptEntryPoint),
			External:    utils.Map(class.External, adaptEntryPoint),
			L1Handler:   utils.Map(class.L1Handler, adaptEntryPoint),
		},
		Prime:                  new(felt.Felt).SetBigInt(class.Prime),
		CompilerVersion:        class.CompilerVersion,
		Bytecode:               class.Bytecode,
		Hints:                  class.Hints,
		BytecodeSegmentLengths: nil, // todo fill this
	}
	return result, nil
}
