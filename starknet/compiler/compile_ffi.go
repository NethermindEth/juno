package compiler

/*
#include <stdint.h>
#include <stdlib.h>
#include <stddef.h>

// Extern function declarations from Rust
extern char compileSierraToCasm(char* sierra_json, char** result);
extern void freeCstr(char* ptr);

// Linker flags for Rust shared library
#cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_compiler_rs
#cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_compiler_rs
*/
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/starknet"
)

// CompileFFI performs Sierra-to-CASM compilation via direct CGo FFI.
func CompileFFI(sierra *starknet.SierraClass) (*starknet.CasmClass, error) {
	sierraJSON, err := json.Marshal(starknet.SierraClass{
		EntryPoints: sierra.EntryPoints,
		Program:     sierra.Program,
		Version:     sierra.Version,
	})
	if err != nil {
		return nil, err
	}

	sierraJSONCstr := C.CString(string(sierraJSON))
	defer C.free(unsafe.Pointer(sierraJSONCstr))

	var result *C.char

	//nolint:gocritic // false positive. It can be either 0 or 1
	success := C.compileSierraToCasm(sierraJSONCstr, &result) == 1
	defer C.freeCstr(result)

	if !success {
		return nil, errors.New(C.GoString(result))
	}

	casmJSON := C.GoString(result)

	var casmClass starknet.CasmClass
	if err := json.Unmarshal([]byte(casmJSON), &casmClass); err != nil {
		return nil, err
	}

	return &casmClass, nil
}
