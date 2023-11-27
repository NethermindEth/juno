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
	"fmt"
	"unsafe"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

func marshalCompiledClass(class core.Class) (json.RawMessage, error) {
	switch c := class.(type) {
	case *core.Cairo0Class:
		compiledCairo0Class, err := core2sn.AdaptCairo0Class(c)
		if err != nil {
			return nil, err
		}
		return json.Marshal(compiledCairo0Class)
	case *core.Cairo1Class:
		compiledCairo1Class := core2sn.AdaptCompiledClass(&c.Compiled)
		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		return json.Marshal(compiledCairo1Class)
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
}

func marshalDeclaredClass(class core.Class) (json.RawMessage, error) {
	var declaredClass any
	var err error

	switch c := class.(type) {
	case *core.Cairo0Class:
		declaredClass, err = core2sn.AdaptCairo0Class(c)
		if err != nil {
			return nil, err
		}
	case *core.Cairo1Class:
		declaredClass = core2sn.AdaptSierraClass(c)
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}

	return json.Marshal(declaredClass)
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
