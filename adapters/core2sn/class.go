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

func AdaptCasmEntryPoint(ep *core.CasmEntryPoint) starknet.CompiledEntryPoint {
	return starknet.CompiledEntryPoint{
		Selector: ep.Selector,
		Builtins: ep.Builtins,
		Offset:   ep.Offset,
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

	feederCompiledClass.EntryPoints.External = make(
		[]starknet.CompiledEntryPoint,
		len(coreCasmClass.External),
	)
	for index, ep := range coreCasmClass.External {
		feederCompiledClass.EntryPoints.External[index] = AdaptCasmEntryPoint(&ep)
	}
	feederCompiledClass.EntryPoints.L1Handler = make(
		[]starknet.CompiledEntryPoint,
		len(coreCasmClass.L1Handler),
	)
	for index, ep := range coreCasmClass.L1Handler {
		feederCompiledClass.EntryPoints.L1Handler[index] = AdaptCasmEntryPoint(&ep)
	}
	feederCompiledClass.EntryPoints.Constructor = make(
		[]starknet.CompiledEntryPoint,
		len(coreCasmClass.Constructor),
	)
	for index, ep := range coreCasmClass.Constructor {
		feederCompiledClass.EntryPoints.Constructor[index] = AdaptCasmEntryPoint(&ep)
	}

	return feederCompiledClass
}

func AdaptSierraEntryPoint(ep *core.SierraEntryPoint) starknet.SierraEntryPoint {
	return starknet.SierraEntryPoint{
		Selector: ep.Selector,
		Index:    ep.Index,
	}
}

func AdaptSierraClass(class *core.SierraClass) starknet.SierraClass {
	constructors := make([]starknet.SierraEntryPoint, len(class.EntryPoints.Constructor))
	for index, ep := range class.EntryPoints.Constructor {
		constructors[index] = AdaptSierraEntryPoint(&ep)
	}
	external := make([]starknet.SierraEntryPoint, len(class.EntryPoints.External))
	for index, ep := range class.EntryPoints.External {
		external[index] = AdaptSierraEntryPoint(&ep)
	}
	handlers := make([]starknet.SierraEntryPoint, len(class.EntryPoints.L1Handler))
	for index, ep := range class.EntryPoints.L1Handler {
		handlers[index] = AdaptSierraEntryPoint(&ep)
	}

	return starknet.SierraClass{
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

func AdaptDeprecatedEntryPoint(ep *core.DeprecatedEntryPoint) starknet.EntryPoint {
	return starknet.EntryPoint{
		Selector: ep.Selector,
		Offset:   ep.Offset,
	}
}

func AdaptDeprecatedCairoClass(
	class *core.DeprecatedCairoClass,
) (starknet.DeprecatedCairoClass, error) {
	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return starknet.DeprecatedCairoClass{}, err
	}
	constructors := make([]starknet.EntryPoint, len(class.Constructors))
	for index, ep := range class.Constructors {
		constructors[index] = AdaptDeprecatedEntryPoint(&ep)
	}
	external := make([]starknet.EntryPoint, len(class.Externals))
	for index, ep := range class.Externals {
		external[index] = AdaptDeprecatedEntryPoint(&ep)
	}
	handlers := make([]starknet.EntryPoint, len(class.L1Handlers))
	for index, ep := range class.L1Handlers {
		handlers[index] = AdaptDeprecatedEntryPoint(&ep)
	}

	return starknet.DeprecatedCairoClass{
		Program: decompressedProgram,
		Abi:     class.Abi,
		EntryPoints: starknet.EntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}, nil
}
