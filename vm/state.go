package vm

/*
#include <stdint.h>
#include <stdlib.h>
#include "vm_ffi.h"
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

//export JunoFree
func JunoFree(ptr unsafe.Pointer) {
	C.free(ptr)
}

//export JunoStateGetStorageAt
func JunoStateGetStorageAt(readerHandle C.uintptr_t, contractAddress, storageLocation, buffer unsafe.Pointer) C.int {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	storageLocationFelt := makeFeltFromPtr(storageLocation)
	val, err := context.state.ContractStorage(contractAddressFelt, storageLocationFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.log.Errorw("JunoStateGetStorageAt failed to read contract storage", "err", err)
			return 0
		}
		val = felt.Zero
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetNonceAt
func JunoStateGetNonceAt(readerHandle C.uintptr_t, contractAddress, buffer unsafe.Pointer) C.int {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	val, err := context.state.ContractNonce(contractAddressFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.log.Errorw("JunoStateGetNonceAt failed to read contract nonce", "err", err)
			return 0
		}
		val = felt.Zero
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetClassHashAt
func JunoStateGetClassHashAt(readerHandle C.uintptr_t, contractAddress, buffer unsafe.Pointer) C.int {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	val, err := context.state.ContractClassHash(contractAddressFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.log.Errorw("JunoStateGetClassHashAt failed to read contract class", "err", err)
			return 0
		}
		val = felt.Zero
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetCompiledClass
func JunoStateGetCompiledClass(readerHandle C.uintptr_t, classHash unsafe.Pointer) C.struct_JunoBytes {
	context := unwrapContext(readerHandle)

	classHashFelt := makeFeltFromPtr(classHash)
	val, err := context.state.Class(classHashFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.log.Errorw("JunoStateGetCompiledClass failed to read class", "err", err)
		}
		return C.struct_JunoBytes{data: nil, len: 0}
	}

	buf, err := marshalClassInfoProtoBytes(val.Class)
	if err != nil {
		context.log.Errorw("JunoStateGetCompiledClass failed to marshal compiled class (proto)", "err", err)
		return C.struct_JunoBytes{data: nil, len: 0}
	}

	cptr := C.malloc(C.size_t(len(buf)))
	if cptr == nil {
		context.log.Errorw("JunoStateGetCompiledClass malloc failed", "size", len(buf))
		return C.struct_JunoBytes{data: nil, len: 0}
	}
	dst := unsafe.Slice((*byte)(cptr), len(buf))
	copy(dst, buf)

	return C.struct_JunoBytes{data: cptr, len: C.size_t(len(buf))}
}

func fillBufferWithFelt(val *felt.Felt, buffer unsafe.Pointer) C.int {
	feltBytes := val.Bytes()
	return C.int(copy(unsafe.Slice((*byte)(buffer), felt.Bytes), feltBytes[:]))
}
