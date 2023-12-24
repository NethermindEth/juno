package core2sn

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

func AdaptCompiledClass(coreCompiledClass *core.CompiledClass) starknet.CompiledClass {
	var feederCompiledClass starknet.CompiledClass
	feederCompiledClass.Bytecode = coreCompiledClass.Bytecode
	feederCompiledClass.PythonicHints = coreCompiledClass.PythonicHints
	feederCompiledClass.CompilerVersion = coreCompiledClass.CompilerVersion
	feederCompiledClass.Hints = coreCompiledClass.Hints
	feederCompiledClass.Prime = "0x" + coreCompiledClass.Prime.Text(felt.Base16)

	adapt := func(ep core.CompiledEntryPoint) starknet.CompiledEntryPoint {
		return starknet.CompiledEntryPoint{
			Selector: ep.Selector,
			Builtins: ep.Builtins,
			Offset:   ep.Offset,
		}
	}
	feederCompiledClass.EntryPoints.External = utils.Map(utils.NonNilSlice(coreCompiledClass.External), adapt)
	feederCompiledClass.EntryPoints.L1Handler = utils.Map(utils.NonNilSlice(coreCompiledClass.L1Handler), adapt)
	feederCompiledClass.EntryPoints.Constructor = utils.Map(utils.NonNilSlice(coreCompiledClass.Constructor), adapt)

	return feederCompiledClass
}

func AdaptSierraClass(class *core.Cairo1Class) *starknet.SierraDefinition {
	adapt := func(ep core.SierraEntryPoint) starknet.SierraEntryPoint {
		return starknet.SierraEntryPoint{
			Selector: ep.Selector,
			Index:    ep.Index,
		}
	}
	constructors := utils.Map(utils.NonNilSlice(class.EntryPoints.Constructor), adapt)
	external := utils.Map(utils.NonNilSlice(class.EntryPoints.External), adapt)
	handlers := utils.Map(utils.NonNilSlice(class.EntryPoints.L1Handler), adapt)

	return &starknet.SierraDefinition{
		Version: class.SemanticVersion,
		Program: class.Program,
		EntryPoints: starknet.SierraEntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}
}

func AdaptCairo0Class(class *core.Cairo0Class) (*starknet.Cairo0Definition, error) {
	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

	adapt := func(ep core.EntryPoint) starknet.EntryPoint {
		return starknet.EntryPoint{
			Selector: ep.Selector,
			Offset:   ep.Offset,
		}
	}
	constructors := utils.Map(utils.NonNilSlice(class.Constructors), adapt)
	external := utils.Map(utils.NonNilSlice(class.Externals), adapt)
	handlers := utils.Map(utils.NonNilSlice(class.L1Handlers), adapt)

	return &starknet.Cairo0Definition{
		Program: decompressedProgram,
		Abi:     class.Abi,
		EntryPoints: starknet.EntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}, nil
}
