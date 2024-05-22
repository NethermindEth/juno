package p2p2core

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/adapters/sn2core"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptClass(class *spec.Class) core.Class {
	if class == nil {
		return nil
	}

	switch cls := class.Class.(type) {
	case *spec.Class_Cairo0:
		cairo0 := cls.Cairo0
		return &core.Cairo0Class{
			Abi:          json.RawMessage(cairo0.Abi),
			Externals:    utils.Map(cairo0.Externals, adaptEntryPoint),
			L1Handlers:   utils.Map(cairo0.L1Handlers, adaptEntryPoint),
			Constructors: utils.Map(cairo0.Constructors, adaptEntryPoint),
			Program:      string(cairo0.Program),
		}
	case *spec.Class_Cairo1:
		cairo1 := cls.Cairo1
		abiHash, err := crypto.StarknetKeccak([]byte(cairo1.Abi))
		if err != nil {
			panic(err)
		}

		program := utils.Map(cairo1.Program, AdaptFelt)
		compiled, err := createCompiledClass(cairo1)
		if err != nil {
			fmt.Println("Version is ", cairo1.ContractClassVersion)
			panic(err)
		}

		return &core.Cairo1Class{
			Abi:     cairo1.Abi,
			AbiHash: abiHash,
			EntryPoints: struct {
				Constructor []core.SierraEntryPoint
				External    []core.SierraEntryPoint
				L1Handler   []core.SierraEntryPoint
			}{
				Constructor: utils.Map(cairo1.EntryPoints.Constructors, adaptSierra),
				External:    utils.Map(cairo1.EntryPoints.Externals, adaptSierra),
				L1Handler:   utils.Map(cairo1.EntryPoints.L1Handlers, adaptSierra),
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

func adaptSierra(e *spec.SierraEntryPoint) core.SierraEntryPoint {
	return core.SierraEntryPoint{
		Index:    e.Index,
		Selector: AdaptFelt(e.Selector),
	}
}

func adaptEntryPoint(e *spec.EntryPoint) core.EntryPoint {
	return core.EntryPoint{
		Selector: AdaptFelt(e.Selector),
		Offset:   new(felt.Felt).SetUint64(e.Offset),
	}
}

func createCompiledClass(cairo1 *spec.Cairo1Class) (*core.CompiledClass, error) {
	if cairo1 == nil {
		return nil, nil
	}

	adapt := func(ep *spec.SierraEntryPoint) starknet.SierraEntryPoint {
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

	compiledClass, err := starknet.Compile(def)
	if err != nil {
		return nil, err
	}

	return sn2core.AdaptCompiledClass(compiledClass)
}
