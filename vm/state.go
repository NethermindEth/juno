package vm

//#include <stdint.h>
//#include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
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
			context.log.Error("JunoStateGetStorageAt failed to read contract storage", utils.SugaredFields("err", err)...)
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
			context.log.Error("JunoStateGetNonceAt failed to read contract nonce", utils.SugaredFields("err", err)...)
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
			context.log.Error("JunoStateGetClassHashAt failed to read contract class", utils.SugaredFields("err", err)...)
			return 0
		}
		val = felt.Zero
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetCompiledClass
func JunoStateGetCompiledClass(readerHandle C.uintptr_t, classHash unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	classHashFelt := makeFeltFromPtr(classHash)
	val, err := context.state.Class(classHashFelt)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.log.Error("JunoStateGetCompiledClass failed to read class", utils.SugaredFields("err", err)...)
		}
		return nil
	}

	compiledClass, err := marshalClassInfo(val.Class)
	if err != nil {
		context.log.Error("JunoStateGetCompiledClass failed to marshal compiled class", utils.SugaredFields("err", err)...)
		return nil
	}

	return unsafe.Pointer(cstring(compiledClass))
}

func fillBufferWithFelt(val *felt.Felt, buffer unsafe.Pointer) C.int {
	feltBytes := val.Bytes()
	return C.int(copy(unsafe.Slice((*byte)(buffer), felt.Bytes), feltBytes[:]))
}

//export JunoStateGetCompiledClassHash
func JunoStateGetCompiledClassHash(
	readerHandle C.uintptr_t,
	classHash,
	buffer unsafe.Pointer,
) C.int {
	context := unwrapContext(readerHandle)

	classHashFelt := makeFeltFromPtr(classHash)
	val, err := context.state.CompiledClassHash((*felt.SierraClassHash)(classHashFelt))
	if err != nil {
		return 0
	}

	return fillBufferWithFelt((*felt.Felt)(&val), buffer)
}

//export JunoStateGetCompiledClassHashV2
func JunoStateGetCompiledClassHashV2(
	readerHandle C.uintptr_t,
	classHash,
	buffer unsafe.Pointer,
) C.int {
	context := unwrapContext(readerHandle)

	classHashFelt := makeFeltFromPtr(classHash)
	val, err := context.state.CompiledClassHashV2((*felt.SierraClassHash)(classHashFelt))
	if err != nil {
		return 0
	}

	return fillBufferWithFelt((*felt.Felt)(&val), buffer)
}
