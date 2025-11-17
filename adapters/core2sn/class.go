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

	if coreCasmClass.BytecodeSegmentLengths.Length != 0 || len(coreCasmClass.BytecodeSegmentLengths.Children) != 0 {
		segmentLengths := AdaptSegmentLengths(coreCasmClass.BytecodeSegmentLengths)
		feederCompiledClass.BytecodeSegmentLengths = &segmentLengths
	}

	feederCompiledClass.EntryPoints.External = make(
		[]starknet.CompiledEntryPoint,
		len(coreCasmClass.External),
	)
	for index := range coreCasmClass.External {
		feederCompiledClass.EntryPoints.External[index] = AdaptCasmEntryPoint(
			&coreCasmClass.External[index],
		)
	}

	feederCompiledClass.EntryPoints.L1Handler = make(
		[]starknet.CompiledEntryPoint,
		len(coreCasmClass.L1Handler),
	)
	for index := range coreCasmClass.L1Handler {
		feederCompiledClass.EntryPoints.L1Handler[index] = AdaptCasmEntryPoint(
			&coreCasmClass.L1Handler[index],
		)
	}

	feederCompiledClass.EntryPoints.Constructor = make(
		[]starknet.CompiledEntryPoint,
		len(coreCasmClass.Constructor),
	)
	for index := range coreCasmClass.Constructor {
		feederCompiledClass.EntryPoints.Constructor[index] = AdaptCasmEntryPoint(
			&coreCasmClass.Constructor[index],
		)
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
	for index := range class.EntryPoints.Constructor {
		constructors[index] = AdaptSierraEntryPoint(&class.EntryPoints.Constructor[index])
	}

	external := make([]starknet.SierraEntryPoint, len(class.EntryPoints.External))
	for index := range class.EntryPoints.External {
		external[index] = AdaptSierraEntryPoint(&class.EntryPoints.External[index])
	}

	handlers := make([]starknet.SierraEntryPoint, len(class.EntryPoints.L1Handler))
	for index := range class.EntryPoints.L1Handler {
		handlers[index] = AdaptSierraEntryPoint(&class.EntryPoints.L1Handler[index])
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
	for index := range class.Constructors {
		constructors[index] = AdaptDeprecatedEntryPoint(&class.Constructors[index])
	}

	external := make([]starknet.EntryPoint, len(class.Externals))
	for index := range class.Externals {
		external[index] = AdaptDeprecatedEntryPoint(&class.Externals[index])
	}

	handlers := make([]starknet.EntryPoint, len(class.L1Handlers))
	for index := range class.L1Handlers {
		handlers[index] = AdaptDeprecatedEntryPoint(&class.L1Handlers[index])
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
