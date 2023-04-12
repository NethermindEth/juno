package vm

//#include <stdint.h>
//#include <stdlib.h>
import "C"

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"unsafe"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
)

//export JunoFree
func JunoFree(ptr unsafe.Pointer) {
	C.free(ptr)
}

//export JunoStateGetStorageAt
func JunoStateGetStorageAt(readerHandle C.uintptr_t, contractAddress, storageLocation unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	storageLocationFelt := makeFeltFromPtr(storageLocation)
	val, err := context.state.ContractStorage(contractAddressFelt, storageLocationFelt)
	if err != nil {
		return nil
	}

	return makePtrFromFelt(val)
}

//export JunoStateGetNonceAt
func JunoStateGetNonceAt(readerHandle C.uintptr_t, contractAddress unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	val, err := context.state.ContractNonce(contractAddressFelt)
	if err != nil {
		return nil
	}

	return makePtrFromFelt(val)
}

//export JunoStateGetClassHashAt
func JunoStateGetClassHashAt(readerHandle C.uintptr_t, contractAddress unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	val, err := context.state.ContractClassHash(contractAddressFelt)
	if err != nil {
		return nil
	}

	return makePtrFromFelt(val)
}

//export JunoStateGetClass
func JunoStateGetClass(readerHandle C.uintptr_t, classHash unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	classHashFelt := makeFeltFromPtr(classHash)
	val, err := context.state.Class(classHashFelt)
	if err != nil {
		return nil
	}

	var vmClass any
	switch class := val.Class.(type) {
	case *core.Cairo0Class:
		vmClass, err = makeDeprecatedVMClass(class)
	case *core.Cairo1Class:
		vmClass = class.Compiled
	default:
		panic("not a class")
	}
	if err != nil {
		return nil
	}

	rustClass, err := json.Marshal(vmClass)
	if err != nil {
		return nil
	}

	return unsafe.Pointer(C.CString(string(rustClass)))
}

func makeDeprecatedVMClass(class *core.Cairo0Class) (*feeder.Cairo0Definition, error) {
	decodedProgram, err := base64.StdEncoding.DecodeString(class.Program)
	if err != nil {
		return nil, err
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(decodedProgram))
	if err != nil {
		return nil, err
	}

	decompressedProgram, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}
	err = gzipReader.Close()
	if err != nil {
		return nil, err
	}

	constructors := make([]feeder.EntryPoint, 0, len(class.Constructors))
	for _, entryPoint := range class.Constructors {
		constructors = append(constructors, feeder.EntryPoint{
			Selector: entryPoint.Selector,
			Offset:   entryPoint.Offset,
		})
	}

	external := make([]feeder.EntryPoint, 0, len(class.Externals))
	for _, entryPoint := range class.Externals {
		external = append(external, feeder.EntryPoint{
			Selector: entryPoint.Selector,
			Offset:   entryPoint.Offset,
		})
	}

	handlers := make([]feeder.EntryPoint, 0, len(class.L1Handlers))
	for _, entryPoint := range class.L1Handlers {
		handlers = append(handlers, feeder.EntryPoint{
			Selector: entryPoint.Selector,
			Offset:   entryPoint.Offset,
		})
	}

	return &feeder.Cairo0Definition{
		Program: decompressedProgram,
		Abi:     class.Abi,
		EntryPoints: feeder.EntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}, nil
}
