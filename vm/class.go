package vm

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

func marshalCompiledClass(class core.Class) (json.RawMessage, error) {
	var compiledClass any
	switch c := class.(type) {
	case *core.Cairo0Class:
		var err error
		compiledClass, err = makeDeprecatedVMClass(c)
		if err != nil {
			return nil, err
		}
	case *core.Cairo1Class:
		compiledClass = c.Compiled
	default:
		return nil, errors.New("not a valid class")
	}

	return json.Marshal(compiledClass)
}

func marshalDeclaredClass(class core.Class) (json.RawMessage, error) {
	var declaredClass any
	var err error

	switch c := class.(type) {
	case *core.Cairo0Class:
		declaredClass, err = makeDeprecatedVMClass(c)
		if err != nil {
			return nil, err
		}
	case *core.Cairo1Class:
		declaredClass = makeSierraClass(c)
	default:
		return nil, errors.New("not a valid class")
	}

	return json.Marshal(declaredClass)
}

func makeDeprecatedVMClass(class *core.Cairo0Class) (*starknet.Cairo0Definition, error) {
	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

	constructors := make([]starknet.EntryPoint, 0, len(class.Constructors))
	for _, entryPoint := range class.Constructors {
		constructors = append(constructors, starknet.EntryPoint{
			Selector: entryPoint.Selector,
			Offset:   entryPoint.Offset,
		})
	}

	external := make([]starknet.EntryPoint, 0, len(class.Externals))
	for _, entryPoint := range class.Externals {
		external = append(external, starknet.EntryPoint{
			Selector: entryPoint.Selector,
			Offset:   entryPoint.Offset,
		})
	}

	handlers := make([]starknet.EntryPoint, 0, len(class.L1Handlers))
	for _, entryPoint := range class.L1Handlers {
		handlers = append(handlers, starknet.EntryPoint{
			Selector: entryPoint.Selector,
			Offset:   entryPoint.Offset,
		})
	}

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

func makeSierraClass(class *core.Cairo1Class) *starknet.SierraDefinition {
	constructors := make([]starknet.SierraEntryPoint, 0, len(class.EntryPoints.Constructor))
	for _, entryPoint := range class.EntryPoints.Constructor {
		constructors = append(constructors, starknet.SierraEntryPoint{
			Selector: entryPoint.Selector,
			Index:    entryPoint.Index,
		})
	}

	external := make([]starknet.SierraEntryPoint, 0, len(class.EntryPoints.External))
	for _, entryPoint := range class.EntryPoints.External {
		external = append(external, starknet.SierraEntryPoint{
			Selector: entryPoint.Selector,
			Index:    entryPoint.Index,
		})
	}

	handlers := make([]starknet.SierraEntryPoint, 0, len(class.EntryPoints.L1Handler))
	for _, entryPoint := range class.EntryPoints.L1Handler {
		handlers = append(handlers, starknet.SierraEntryPoint{
			Selector: entryPoint.Selector,
			Index:    entryPoint.Index,
		})
	}

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
