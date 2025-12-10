package p2p2core

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/class"
)

func AdaptSierraClass(cairo1 *class.Cairo1Class) (core.SierraClass, error) {
	abiHash := crypto.StarknetKeccak([]byte(cairo1.Abi))

	program := utils.Map(cairo1.Program, AdaptFelt)
	compiled, err := createCompiledClass(cairo1)
	if err != nil {
		return core.SierraClass{}, fmt.Errorf("invalid compiled class: %w", err)
	}

	adaptEP := func(points []*class.SierraEntryPoint) []core.SierraEntryPoint {
		// usage of NonNilSlice is essential because relevant core class fields are non nil
		return utils.Map(utils.NonNilSlice(points), adaptSierra)
	}

	programHash := crypto.PoseidonArray(program...)
	entryPoints := cairo1.EntryPoints
	return core.SierraClass{
		Abi:     cairo1.Abi,
		AbiHash: &abiHash,
		EntryPoints: struct {
			Constructor []core.SierraEntryPoint
			External    []core.SierraEntryPoint
			L1Handler   []core.SierraEntryPoint
		}{
			Constructor: adaptEP(entryPoints.Constructors),
			External:    adaptEP(entryPoints.Externals),
			L1Handler:   adaptEP(entryPoints.L1Handlers),
		},
		Program:         program,
		ProgramHash:     &programHash,
		SemanticVersion: cairo1.ContractClassVersion,
		Compiled:        compiled,
	}, nil
}

func AdaptClass(cls *class.Class) (core.ClassDefinition, error) {
	if cls == nil {
		return nil, nil
	}

	switch cls := cls.Class.(type) {
	case *class.Class_Cairo0:
		adaptEP := func(points []*class.EntryPoint) []core.DeprecatedEntryPoint {
			// usage of NonNilSlice is essential because relevant core class fields are non nil
			return utils.Map(utils.NonNilSlice(points), adaptEntryPoint)
		}

		deprecatedCairo := cls.Cairo0
		return &core.DeprecatedCairoClass{
			Abi:          json.RawMessage(deprecatedCairo.Abi),
			Externals:    adaptEP(deprecatedCairo.Externals),
			L1Handlers:   adaptEP(deprecatedCairo.L1Handlers),
			Constructors: adaptEP(deprecatedCairo.Constructors),
			Program:      deprecatedCairo.Program,
		}, nil
	case *class.Class_Cairo1:
		cairoClass, err := AdaptSierraClass(cls.Cairo1)
		return &cairoClass, err
	default:
		return nil, fmt.Errorf("unsupported class %T", cls)
	}
}

func adaptSierra(e *class.SierraEntryPoint) core.SierraEntryPoint {
	return core.SierraEntryPoint{
		Index:    e.Index,
		Selector: AdaptFelt(e.Selector),
	}
}

func adaptEntryPoint(e *class.EntryPoint) core.DeprecatedEntryPoint {
	return core.DeprecatedEntryPoint{
		Selector: AdaptFelt(e.Selector),
		Offset:   new(felt.Felt).SetUint64(e.Offset),
	}
}

func createCompiledClass(cairo1 *class.Cairo1Class) (*core.CasmClass, error) {
	if cairo1 == nil {
		return nil, nil
	}

	adapt := func(ep *class.SierraEntryPoint) starknet.SierraEntryPoint {
		return starknet.SierraEntryPoint{
			Index:    ep.Index,
			Selector: AdaptFelt(ep.Selector),
		}
	}
	ep := cairo1.EntryPoints
	def := &starknet.SierraClass{
		Abi: cairo1.Abi,
		EntryPoints: starknet.SierraEntryPoints{
			// WARNING: usage of utils.NonNilSlice is essential, otherwise compilation will finish with errors
			// todo move NonNilSlice to Compile ?
			Constructor: utils.Map(utils.NonNilSlice(ep.Constructors), adapt),
			External:    utils.Map(utils.NonNilSlice(ep.Externals), adapt),
			L1Handler:   utils.Map(utils.NonNilSlice(ep.L1Handlers), adapt),
		},
		Program: utils.Map(cairo1.Program, AdaptFelt),
		Version: cairo1.ContractClassVersion,
	}

	compiledClass, err := compiler.Compile(def)
	if err != nil {
		return nil, err
	}
	coreCasmClass, err := sn2core.AdaptCasmClass(compiledClass)
	return &coreCasmClass, err
}
