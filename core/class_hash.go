package core

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
//
// extern void Cairo0ClassHash(char* class_json_str, char* hash);
// #cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_core_rs
// #cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_core_rs
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

func deprecatedCairoClassHash(class *DeprecatedCairoClass) (felt.Felt, error) {
	definition, err := makeDeprecatedVMClass(class)
	if err != nil {
		return felt.Felt{}, err
	}

	classJSON, err := json.Marshal(definition)
	if err != nil {
		return felt.Felt{}, err
	}
	classJSONCStr := cstring(classJSON)

	var hash felt.Felt
	hashBytes := hash.Bytes()

	C.Cairo0ClassHash(classJSONCStr, (*C.char)(unsafe.Pointer(&hashBytes[0])))
	hash.SetBytes(hashBytes[:])
	C.free(unsafe.Pointer(classJSONCStr))
	if hash.IsZero() {
		return felt.Felt{}, errors.New("failed to calculate class hash")
	}
	return hash, nil
}

func makeDeprecatedVMClass(class *DeprecatedCairoClass) (*starknet.DeprecatedCairoClass, error) {
	adaptEntryPoint := func(ep DeprecatedEntryPoint) starknet.EntryPoint {
		return starknet.EntryPoint{
			Selector: ep.Selector,
			Offset:   ep.Offset,
		}
	}

	constructors := utils.Map(utils.NonNilSlice(class.Constructors), adaptEntryPoint)
	external := utils.Map(utils.NonNilSlice(class.Externals), adaptEntryPoint)
	handlers := utils.Map(utils.NonNilSlice(class.L1Handlers), adaptEntryPoint)

	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

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

// cstring creates a null-terminated C string from the given byte slice.
// the caller is responsible for freeing the underlying memory
func cstring(data []byte) *C.char {
	str := unsafe.String(unsafe.SliceData(data), len(data))
	return C.CString(str)
}
