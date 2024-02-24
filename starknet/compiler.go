package starknet

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
// extern char* compileSierraToCasm(char* sierra_json);
// extern void freeCstr(char* ptr);
//
// #cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_compiler_rs -ldl -lm
// #cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_compiler_rs -ldl -lm
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"
)

func Compile(sierra *SierraDefinition) (*CompiledClass, error) {
	sierraJSON, err := json.Marshal(SierraDefinition{
		EntryPoints: sierra.EntryPoints,
		Program:     sierra.Program,
		Version:     sierra.Version,
	})
	if err != nil {
		return nil, err
	}

	sierraJSONCstr := C.CString(string(sierraJSON))
	defer func() {
		C.free(unsafe.Pointer(sierraJSONCstr))
	}()

	casmJSONOrErrorCstr := C.compileSierraToCasm(sierraJSONCstr)
	casmJSONOrError := C.GoString(casmJSONOrErrorCstr)
	C.freeCstr(casmJSONOrErrorCstr)

	var casmClass CompiledClass
	if err = json.Unmarshal([]byte(casmJSONOrError), &casmClass); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return nil, errors.New(casmJSONOrError)
		}
		return nil, err
	}

	return &casmClass, nil
}
