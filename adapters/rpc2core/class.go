package rpc2core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils"
)

func AdaptClass(
	class *rpcv10.Class,
	casm *rpcv10.CompiledCasmResponse,
) (core.ClassDefinition, error) {
	if class == nil {
		return nil, errors.New("nil class")
	}
	if !IsSierraClass(class) {
		return adaptDeprecatedClass(class)
	}
	return adaptSierraClass(class, casm)
}

func IsSierraClass(class *rpcv10.Class) bool {
	return class.Program == ""
}

func adaptSierraClass(
	class *rpcv10.Class,
	casm *rpcv10.CompiledCasmResponse,
) (core.ClassDefinition, error) {
	abi, err := sierraABI(class.Abi)
	if err != nil {
		return nil, err
	}
	program := class.SierraProgram
	// The program opens with its three version felts, except version 0.1.0 which is a single felt.
	if len(program) < 3 && (len(program) == 0 || !program[0].Equal(&core.SierraVersion010)) {
		return nil, errors.New("sierra program size is too small")
	}
	entryPoints, err := adaptSierraEntryPoints(&class.EntryPoints)
	if err != nil {
		return nil, err
	}

	if casm == nil {
		return nil, errors.New("sierra class is missing its compiled class")
	}
	compiled, err := adaptCasm(casm)
	if err != nil {
		return nil, err
	}

	programHash := crypto.PoseidonArray(program)
	abiHash := crypto.StarknetKeccak([]byte(abi))
	return &core.SierraClass{
		Abi:             abi,
		AbiHash:         &abiHash,
		EntryPoints:     entryPoints,
		Program:         program,
		ProgramHash:     &programHash,
		SemanticVersion: class.ContractClassVersion,
		Compiled:        compiled,
	}, nil
}

func sierraABI(abi any) (string, error) {
	switch v := abi.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("sierra class abi is %T, want string", abi)
	}
}

func adaptSierraEntryPoints(
	entryPoints *rpcv10.ClassEntryPointsByType,
) (core.SierraEntryPointsByType, error) {
	adapt := func(kind string, points []rpcv10.ClassEntryPoint) ([]core.SierraEntryPoint, error) {
		adapted := make([]core.SierraEntryPoint, len(points))
		for i, point := range points {
			switch {
			case point.Index == nil:
				return nil, fmt.Errorf("%s entry point %d is missing function_idx", kind, i)
			case point.Selector == nil:
				return nil, fmt.Errorf("%s entry point %d is missing selector", kind, i)
			}
			adapted[i] = core.SierraEntryPoint{Index: *point.Index, Selector: point.Selector}
		}
		return adapted, nil
	}

	constructor, err := adapt("CONSTRUCTOR", entryPoints.Constructor)
	if err != nil {
		return core.SierraEntryPointsByType{}, err
	}
	external, err := adapt("EXTERNAL", entryPoints.External)
	if err != nil {
		return core.SierraEntryPointsByType{}, err
	}
	l1Handler, err := adapt("L1_HANDLER", entryPoints.L1Handler)
	if err != nil {
		return core.SierraEntryPointsByType{}, err
	}
	return core.SierraEntryPointsByType{
		Constructor: constructor,
		External:    external,
		L1Handler:   l1Handler,
	}, nil
}

func adaptCasm(casm *rpcv10.CompiledCasmResponse) (*core.CasmClass, error) {
	prime, ok := new(big.Int).SetString(casm.Prime, 0)
	if !ok {
		return nil, fmt.Errorf("compiled class prime %q is not a number", casm.Prime)
	}
	segments, err := adaptSegmentLengths(casm.BytecodeSegmentLengths, len(casm.Bytecode))
	if err != nil {
		return nil, err
	}
	return &core.CasmClass{
		Bytecode:               casm.Bytecode,
		PythonicHints:          nil,
		CompilerVersion:        casm.CompilerVersion,
		Hints:                  casm.Hints,
		Prime:                  prime,
		External:               utils.Map(casm.EntryPointsByType.External, adaptCasmEntryPoint),
		L1Handler:              utils.Map(casm.EntryPointsByType.L1Handler, adaptCasmEntryPoint),
		Constructor:            utils.Map(casm.EntryPointsByType.Constructor, adaptCasmEntryPoint),
		BytecodeSegmentLengths: segments,
	}, nil
}

func adaptCasmEntryPoint(point rpcv10.EntryPoint) core.CasmEntryPoint {
	return core.CasmEntryPoint{
		Offset:   point.Offset,
		Builtins: point.Builtins,
		Selector: &point.Selector,
	}
}

// adaptSegmentLengths rebuilds the compiler's segment tree from the flat list the spec
// carries. The list cannot express nesting, so every element becomes a leaf, except that
// a single element covering the whole bytecode is the bytecode's own length (a leaf node
// with no children), which is how the feeder gateway serves single-segment classes.
// Absent, and Juno's [0] next to non-empty bytecode, mean the class has no segments.
func adaptSegmentLengths(lengths []int, bytecodeLength int) (core.SegmentLengths, error) {
	if len(lengths) == 0 || (len(lengths) == 1 && lengths[0] == 0 && bytecodeLength > 0) {
		return core.SegmentLengths{Length: 0, Children: nil}, nil
	}
	if len(lengths) == 1 && lengths[0] == bytecodeLength {
		return core.SegmentLengths{Length: uint64(bytecodeLength), Children: nil}, nil
	}

	children := make([]core.SegmentLengths, len(lengths))
	total := 0
	for i, length := range lengths {
		if length < 0 {
			return core.SegmentLengths{}, fmt.Errorf("bytecode segment %d has negative length %d", i, length)
		}
		// Bounding each length before adding it keeps the sum from overflowing.
		if length > bytecodeLength-total {
			return core.SegmentLengths{}, fmt.Errorf(
				"bytecode segment %d has length %d but only %d bytecode felts remain",
				i, length, bytecodeLength-total,
			)
		}
		children[i] = core.SegmentLengths{Length: uint64(length), Children: nil}
		total += length
	}
	if total != bytecodeLength {
		return core.SegmentLengths{}, fmt.Errorf(
			"bytecode segment lengths sum to %d felts but the bytecode has %d", total, bytecodeLength,
		)
	}
	return core.SegmentLengths{Length: 0, Children: children}, nil
}

func adaptDeprecatedClass(class *rpcv10.Class) (core.ClassDefinition, error) {
	var abi json.RawMessage
	if class.Abi != nil {
		var err error
		abi, err = json.Marshal(class.Abi)
		if err != nil {
			return nil, fmt.Errorf("encoding legacy class abi: %w", err)
		}
	}
	externals, err := adaptDeprecatedEntryPoints("EXTERNAL", class.EntryPoints.External)
	if err != nil {
		return nil, err
	}
	l1Handlers, err := adaptDeprecatedEntryPoints("L1_HANDLER", class.EntryPoints.L1Handler)
	if err != nil {
		return nil, err
	}
	constructors, err := adaptDeprecatedEntryPoints("CONSTRUCTOR", class.EntryPoints.Constructor)
	if err != nil {
		return nil, err
	}
	// The spec's program is the base64 gzip core stores, and the class hash decodes and
	// bounds it, so it is kept as served.
	return &core.DeprecatedCairoClass{
		Abi:          abi,
		Externals:    externals,
		L1Handlers:   l1Handlers,
		Constructors: constructors,
		Program:      class.Program,
	}, nil
}

// adaptDeprecatedEntryPoints never returns a nil list: the class hash and encoding expect
// present, possibly empty, entry point lists.
func adaptDeprecatedEntryPoints(
	kind string,
	points []rpcv10.ClassEntryPoint,
) ([]core.DeprecatedEntryPoint, error) {
	adapted := make([]core.DeprecatedEntryPoint, len(points))
	for i, point := range points {
		switch {
		case point.Selector == nil:
			return nil, fmt.Errorf("%s entry point %d is missing selector", kind, i)
		case point.Offset == nil:
			return nil, fmt.Errorf("%s entry point %d is missing offset", kind, i)
		}
		adapted[i] = core.DeprecatedEntryPoint{Selector: point.Selector, Offset: point.Offset}
	}
	return adapted, nil
}
