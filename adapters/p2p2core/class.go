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

func AdaptSierraClass(sierraClass *class.Cairo1Class) (core.SierraClass, error) {
	abiHash := crypto.StarknetKeccak([]byte(sierraClass.Abi))

	program := utils.Map(sierraClass.Program, AdaptFelt)

	casmClass, err := compileToCasm(sierraClass)
	if err != nil {
		return core.SierraClass{}, fmt.Errorf("invalid compiled class: %w", err)
	}

	entryPoints := sierraClass.EntryPoints
	return core.SierraClass{
		Abi:     sierraClass.Abi,
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
		SemanticVersion: sierraClass.ContractClassVersion,
		Compiled:        &casmClass,
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
	for i := range points {
		sierraEntryPoints[i] = core.SierraEntryPoint{
			Index: points[i].Index,
			// todo(rdr): look for a way to get rid of this AdaptFelt or change it
			Selector: AdaptFelt(points[i].Selector),
		}
	}
	return sierraEntryPoints
}

func adaptDeprecatedEntryPoints(points []*class.EntryPoint) []core.DeprecatedEntryPoint {
	deprecatedEntryPoints := make([]core.DeprecatedEntryPoint, len(points))
	for i := range points {
		deprecatedEntryPoints[i] = core.DeprecatedEntryPoint{
			// todo(rdr): look for a way to get rid of this AdaptFelt or change it
			Selector: AdaptFelt(points[i].Selector),
			// todo(rdr): wHy do we store this as a felt instead of as a uint64
			Offset: felt.NewFromUint64[felt.Felt](points[i].Offset),
		}
	}
	return deprecatedEntryPoints
}

// todo(rdr): There is a p2p to starknet conversion here
//            Can this whole code be differnet, Like adapting from p2p to core and from core to sn
//            Or write a dedicated adapter. I think this option might be the best one

func compileToCasm(cairo1 *class.Cairo1Class) (core.CasmClass, error) {
	// todo(rdr): write a dedicatd function for this
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
			// todo(rdr): remove the this functional programming which doesn't pair well
			//            with go (both style and performance wise)
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
