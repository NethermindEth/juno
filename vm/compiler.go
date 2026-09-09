package vm

/*
#include "vm_ffi.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/starknet"
)

// CompileSierraToCasm compiles a Sierra class to CASM in-process via the Rust FFI.
func CompileSierraToCasm(sierra *starknet.SierraClass) (*starknet.CasmClass, error) {
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
