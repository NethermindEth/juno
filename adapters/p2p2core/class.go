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
	compiled, err := compileToCasm(cairo1)
	if err != nil {
		return core.SierraClass{}, fmt.Errorf("invalid compiled class: %w", err)
	}

	entryPoints := cairo1.EntryPoints
	return core.SierraClass{
		Abi:     cairo1.Abi,
		AbiHash: abiHash,
		EntryPoints: struct {
			Constructor []core.SierraEntryPoint
			External    []core.SierraEntryPoint
			L1Handler   []core.SierraEntryPoint
		}{
			Constructor: adaptSierraEntryPoints(entryPoints.Constructors),
			External:    adaptSierraEntryPoints(entryPoints.Externals),
			L1Handler:   adaptSierraEntryPoints(entryPoints.L1Handlers),
		},
		Program:         program,
		ProgramHash:     crypto.PoseidonArray(program...),
		SemanticVersion: cairo1.ContractClassVersion,
		Compiled:        &compiled,
	}, nil
}

func AdaptClassDefinition(cls *class.Class) (core.ClassDefinition, error) {
	if cls == nil {
		return nil, nil
	}

	switch cls := cls.Class.(type) {
	case *class.Class_Cairo0:

		deprecatedCairo := cls.Cairo0
		return &core.DeprecatedCairoClass{
			Abi:          json.RawMessage(deprecatedCairo.Abi),
			Externals:    adaptDeprecatedEntryPoints(deprecatedCairo.Externals),
			L1Handlers:   adaptDeprecatedEntryPoints(deprecatedCairo.L1Handlers),
			Constructors: adaptDeprecatedEntryPoints(deprecatedCairo.Constructors),
			Program:      deprecatedCairo.Program,
		}, nil
	case *class.Class_Cairo1:
		cairoClass, err := AdaptSierraClass(cls.Cairo1)
		return &cairoClass, err
	default:
		return nil, fmt.Errorf("unsupported class %T", cls)
	}
}

func adaptSierraEntryPoints(points []*class.SierraEntryPoint) []core.SierraEntryPoint {
	sierraEntryPoints := make([]core.SierraEntryPoint, len(points))
	for index := range points {
		sierraEntryPoints[index] = adaptSierraEntryPoint(points[index])
	}
	return sierraEntryPoints
}

func adaptSierraEntryPoint(e *class.SierraEntryPoint) core.SierraEntryPoint {
	return core.SierraEntryPoint{
		Index:    e.Index,
		Selector: AdaptFelt(e.Selector),
	}
}

func adaptDeprecatedEntryPoints(points []*class.EntryPoint) []core.DeprecatedEntryPoint {
	deprecatedEntryPoints := make([]core.DeprecatedEntryPoint, len(points))
	for index := range points {
		deprecatedEntryPoints[index] = adaptEntryPoint(points[index])
	}
	return deprecatedEntryPoints
}

func adaptEntryPoint(e *class.EntryPoint) core.DeprecatedEntryPoint {
	return core.DeprecatedEntryPoint{
		Selector: AdaptFelt(e.Selector),
		Offset:   new(felt.Felt).SetUint64(e.Offset),
	}
}

// todo should this be a method of class.Cairo1Class

func compileToCasm(cairo1 *class.Cairo1Class) (core.CasmClass, error) {
	if cairo1 == nil {
		//nolint:exhaustruct // intentionally returning an empty CasmClass
		return core.CasmClass{}, nil
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
		return core.CasmClass{}, err
	}

	return sn2core.AdaptCompiledClass(compiledClass)
}
