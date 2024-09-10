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
)

func cairo0ClassHash(class *Cairo0Class) (*felt.Felt, error) {
	definition, err := makeDeprecatedVMClass(class)
	if err != nil {
		return nil, err
	}

	classJSON, err := json.Marshal(definition)
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

// cstring creates a null-terminated C string from the given byte slice.
// the caller is responsible for freeing the underlying memory
func cstring(data []byte) *C.char {
	str := unsafe.String(unsafe.SliceData(data), len(data))
	return C.CString(str)
}
