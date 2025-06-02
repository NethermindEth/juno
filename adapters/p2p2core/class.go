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

func AdaptClass(cls *class.Class) core.Class {
	if cls == nil {
		return nil
	}

	switch cls := cls.Class.(type) {
	case *class.Class_Cairo0:
		adaptEP := func(points []*class.EntryPoint) []core.EntryPoint {
			// usage of NonNilSlice is essential because relevant core class fields are non nil
			return utils.Map(utils.NonNilSlice(points), adaptEntryPoint)
		}

		cairo0 := cls.Cairo0
		return &core.Cairo0Class{
			Abi:          json.RawMessage(cairo0.Abi),
			Externals:    adaptEP(cairo0.Externals),
			L1Handlers:   adaptEP(cairo0.L1Handlers),
			Constructors: adaptEP(cairo0.Constructors),
			Program:      cairo0.Program,
		}
	case *class.Class_Cairo1:
		cairo1 := cls.Cairo1
		abiHash := crypto.StarknetKeccak([]byte(cairo1.Abi))

		program := utils.Map(cairo1.Program, AdaptFelt)
		compiled, err := createCompiledClass(cairo1)
		if err != nil {
			panic(err)
		}

		adaptEP := func(points []*class.SierraEntryPoint) []core.SierraEntryPoint {
			// usage of NonNilSlice is essential because relevant core class fields are non nil
			return utils.Map(utils.NonNilSlice(points), adaptSierra)
		}

		entryPoints := cairo1.EntryPoints
		return &core.Cairo1Class{
			Abi:     cairo1.Abi,
			AbiHash: abiHash,
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
			ProgramHash:     crypto.PoseidonArray(program...),
			SemanticVersion: cairo1.ContractClassVersion,
			Compiled:        compiled,
		}
	default:
		panic(fmt.Errorf("unsupported class %T", cls))
	}
}

func adaptSierra(e *class.SierraEntryPoint) core.SierraEntryPoint {
	return core.SierraEntryPoint{
		Index:    e.Index,
		Selector: AdaptFelt(e.Selector),
	}
}

func adaptEntryPoint(e *class.EntryPoint) core.EntryPoint {
	return core.EntryPoint{
		Selector: AdaptFelt(e.Selector),
		Offset:   new(felt.Felt).SetUint64(e.Offset),
	}
}

func createCompiledClass(cairo1 *class.Cairo1Class) (*core.CompiledClass, error) {
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
	def := &starknet.SierraDefinition{
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

	return sn2core.AdaptCompiledClass(compiledClass)
}
