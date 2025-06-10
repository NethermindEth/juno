package p2p2consensus

import (
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/class"
)

func AdaptClass(cls *class.Cairo1Class) core.Class {
	if cls == nil {
		return nil
	}

	abiHash := crypto.StarknetKeccak([]byte(cls.Abi))

	program := utils.Map(cls.Program, p2p2core.AdaptFelt)
	compiled, err := createCompiledClass(cls)
	if err != nil {
		panic(err)
	}

	adaptEP := func(points []*class.SierraEntryPoint) []core.SierraEntryPoint {
		// usage of NonNilSlice is essential because relevant core class fields are non nil
		return utils.Map(utils.NonNilSlice(points), adaptSierra)
	}

	entryPoints := cls.EntryPoints
	return &core.Cairo1Class{
		Abi:     cls.Abi,
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
		SemanticVersion: cls.ContractClassVersion,
		Compiled:        compiled,
	}
}

func adaptSierra(e *class.SierraEntryPoint) core.SierraEntryPoint {
	return core.SierraEntryPoint{
		Index:    e.Index,
		Selector: p2p2core.AdaptFelt(e.Selector),
	}
}

func createCompiledClass(cairo1 *class.Cairo1Class) (*core.CompiledClass, error) {
	if cairo1 == nil {
		return nil, nil
	}

	adapt := func(ep *class.SierraEntryPoint) starknet.SierraEntryPoint {
		return starknet.SierraEntryPoint{
			Index:    ep.Index,
			Selector: p2p2core.AdaptFelt(ep.Selector),
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
		Program: utils.Map(cairo1.Program, p2p2core.AdaptFelt),
		Version: cairo1.ContractClassVersion,
	}

	compiledClass, err := compiler.Compile(def)
	if err != nil {
		return nil, err
	}

	return sn2core.AdaptCompiledClass(compiledClass)
}
