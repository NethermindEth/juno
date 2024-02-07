package p2p2core

import (
	"encoding/json"
	"fmt"

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
			Abi:          cairo0.Abi,
			Externals:    utils.Map(cairo0.Externals, adaptEntryPoint),
			L1Handlers:   utils.Map(cairo0.L1Handlers, adaptEntryPoint),
			Constructors: utils.Map(cairo0.Constructors, adaptEntryPoint),
			Program:      string(cairo0.Program),
		}
	case *spec.Class_Cairo1:
		cairo1 := cls.Cairo1
		abiHash, err := crypto.StarknetKeccak(cairo1.Abi)
		if err != nil {
			panic(err)
		}

		// Todo: remove once compiled class hash is added to p2p spec.
		compiledC := new(core.CompiledClass)
		if err := json.Unmarshal(cairo1.Compiled, compiledC); err != nil {
			panic(fmt.Errorf("unable to unmarshal json compiled class: %v", err))
		}

		program := utils.Map(cairo1.Program, AdaptFelt)
		return &core.Cairo1Class{
			Abi:     string(cairo1.Abi),
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
			Compiled:        compiledC,
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
		Offset:   AdaptFelt(e.Offset),
	}
}
