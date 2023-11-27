package vm

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
//
// extern void Cairo0ClassHash(char* class_json_str, char* hash);
//
// #cgo vm_debug LDFLAGS:  -L./rust/target/debug -ljuno_starknet_rs -lm -ldl
// #cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_rs -lm -ldl
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

func marshalCompiledClass(class core.Class) (json.RawMessage, error) {
	switch c := class.(type) {
	case *core.Cairo0Class:
		compiledCairo0Class, err := makeDeprecatedVMClass(c)
		if err != nil {
			return nil, err
		}
		return json.Marshal(compiledCairo0Class)
	case *core.Cairo1Class:
		compiledCairo1Class := makeCairo1CompiledClass(&c.Compiled)
		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		return json.Marshal(compiledCairo1Class)
	default:
		return nil, errors.New("not a valid class")
	}
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

func makeCairo1CompiledClass(coreCompiledClass *core.CompiledClass) starknet.CompiledClass {
	feederCompiledClass := new(starknet.CompiledClass)
	feederCompiledClass.Bytecode = coreCompiledClass.Bytecode
	feederCompiledClass.PythonicHints = coreCompiledClass.PythonicHints
	feederCompiledClass.CompilerVersion = coreCompiledClass.CompilerVersion
	feederCompiledClass.Hints = coreCompiledClass.Hints
	feederCompiledClass.Prime = "0x" + coreCompiledClass.Prime.Text(16) //nolint:gomnd

	feederCompiledClass.EntryPoints.External = make([]starknet.CompiledEntryPoint, len(coreCompiledClass.External))
	for i, external := range coreCompiledClass.External {
		feederCompiledClass.EntryPoints.External[i] = starknet.CompiledEntryPoint{
			Selector: external.Selector,
			Builtins: external.Builtins,
			Offset:   external.Offset,
		}
	}

	feederCompiledClass.EntryPoints.L1Handler = make([]starknet.CompiledEntryPoint, len(coreCompiledClass.L1Handler))
	for i, external := range coreCompiledClass.L1Handler {
		feederCompiledClass.EntryPoints.L1Handler[i] = starknet.CompiledEntryPoint{
			Selector: external.Selector,
			Builtins: external.Builtins,
			Offset:   external.Offset,
		}
	}

	feederCompiledClass.EntryPoints.Constructor = make([]starknet.CompiledEntryPoint, len(coreCompiledClass.Constructor))
	for i, external := range coreCompiledClass.Constructor {
		feederCompiledClass.EntryPoints.Constructor[i] = starknet.CompiledEntryPoint{
			Selector: external.Selector,
			Builtins: external.Builtins,
			Offset:   external.Offset,
		}
	}

	return *feederCompiledClass
}

func Cairo0ClassHash(class *core.Cairo0Class) (*felt.Felt, error) {
	classJSON, err := marshalDeclaredClass(class)
	if err != nil {
		return nil, err
	}
	classJSONCStr := cstring(classJSON)

	var hash felt.Felt
	hashBytes := hash.Bytes()

	C.Cairo0ClassHash(classJSONCStr, (*C.char)(unsafe.Pointer(&hashBytes[0])))
	hash.SetBytes(hashBytes[:])
	C.free(unsafe.Pointer(classJSONCStr))
	if hash.IsZero() {
		return nil, errors.New("failed to calculate class hash")
	}
	return &hash, nil
}
