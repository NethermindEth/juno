package vm

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/adapters/core2sn"
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
	constructors := utils.Map(utils.NonNilSlice(class.Constructors), core2sn.AdaptEntryPoint)
	external := utils.Map(utils.NonNilSlice(class.Externals), core2sn.AdaptEntryPoint)
	handlers := utils.Map(utils.NonNilSlice(class.L1Handlers), core2sn.AdaptEntryPoint)

	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
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
	constructors := utils.Map(utils.NonNilSlice(class.EntryPoints.Constructor), core2sn.AdaptSierraEntryPoint)
	external := utils.Map(utils.NonNilSlice(class.EntryPoints.External), core2sn.AdaptSierraEntryPoint)
	handlers := utils.Map(utils.NonNilSlice(class.EntryPoints.L1Handler), core2sn.AdaptSierraEntryPoint)

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
