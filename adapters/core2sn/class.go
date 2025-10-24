package core2sn

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

func AdaptSegmentLengths(l core.SegmentLengths) starknet.SegmentLengths {
	return starknet.SegmentLengths{
		Length:   l.Length,
		Children: utils.Map(l.Children, AdaptSegmentLengths),
	}
}

func AdaptCasmClass(coreCasmClass *core.CasmClass) starknet.CasmClass {
	var feederCompiledClass starknet.CasmClass
	feederCompiledClass.Bytecode = coreCasmClass.Bytecode
	feederCompiledClass.PythonicHints = coreCasmClass.PythonicHints
	feederCompiledClass.CompilerVersion = coreCasmClass.CompilerVersion
	feederCompiledClass.Hints = coreCasmClass.Hints
	feederCompiledClass.Prime = utils.ToHex(coreCasmClass.Prime)
	feederCompiledClass.BytecodeSegmentLengths = AdaptSegmentLengths(
		coreCasmClass.BytecodeSegmentLengths,
	)

	adapt := func(ep core.CasmEntryPoint) starknet.CompiledEntryPoint {
		return starknet.CompiledEntryPoint{
			Selector: ep.Selector,
			Builtins: ep.Builtins,
			Offset:   ep.Offset,
		}
	}
	feederCompiledClass.EntryPoints.External = utils.Map(
		utils.NonNilSlice(coreCasmClass.External),
		adapt,
	)
	feederCompiledClass.EntryPoints.L1Handler = utils.Map(
		utils.NonNilSlice(coreCasmClass.L1Handler),
		adapt,
	)
	feederCompiledClass.EntryPoints.Constructor = utils.Map(
		utils.NonNilSlice(coreCasmClass.Constructor),
		adapt,
	)

	return feederCompiledClass
}

func AdaptSierraClass(class *core.SierraClass) *starknet.SierraClass {
	adapt := func(ep core.SierraEntryPoint) starknet.SierraEntryPoint {
		return starknet.SierraEntryPoint{
			Selector: ep.Selector,
			Index:    ep.Index,
		}
	}
	constructors := utils.Map(utils.NonNilSlice(class.EntryPoints.Constructor), adapt)
	external := utils.Map(utils.NonNilSlice(class.EntryPoints.External), adapt)
	handlers := utils.Map(utils.NonNilSlice(class.EntryPoints.L1Handler), adapt)

	return &starknet.SierraClass{
		Abi:     class.Abi,
		Version: class.SemanticVersion,
		Program: class.Program,
		EntryPoints: starknet.SierraEntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}
}

func AdaptDeprecatedCairoClass(
	class *core.DeprecatedCairoClass,
) (*starknet.DeprecatedCairoClass, error) {
	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

	adapt := func(ep core.DeprecatedEntryPoint) starknet.EntryPoint {
		return starknet.EntryPoint{
			Selector: ep.Selector,
			Offset:   ep.Offset,
		}
	}
	constructors := utils.Map(utils.NonNilSlice(class.Constructors), adapt)
	external := utils.Map(utils.NonNilSlice(class.Externals), adapt)
	handlers := utils.Map(utils.NonNilSlice(class.L1Handlers), adapt)

	return &starknet.DeprecatedCairoClass{
		Program: decompressedProgram,
		Abi:     class.Abi,
		EntryPoints: starknet.EntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}, nil
}
