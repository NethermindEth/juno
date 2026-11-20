package vm

//#include <stdint.h>
//#include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"go.uber.org/zap"
)

//export JunoFree
func JunoFree(ptr unsafe.Pointer) {
	C.free(ptr)
}

//export JunoStateGetStorageAt
func JunoStateGetStorageAt(readerHandle C.uintptr_t, contractAddress, storageLocation, buffer unsafe.Pointer) C.int {
	context := unwrapContext(readerHandle)

	val, err := context.state.ContractStorage(
		makeFeltFromPtr[felt.Address](contractAddress),
		makeFeltFromPtr[felt.Felt](storageLocation),
	)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.logger.Error("JunoStateGetStorageAt failed to read contract storage", zap.Error(err))
			return 0
		}
		val = felt.Zero[felt.Felt]()
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetNonceAt
func JunoStateGetNonceAt(readerHandle C.uintptr_t, contractAddress, buffer unsafe.Pointer) C.int {
	context := unwrapContext(readerHandle)

	val, err := context.state.ContractNonce(makeFeltFromPtr[felt.Address](contractAddress))
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.logger.Error("JunoStateGetNonceAt failed to read contract nonce", zap.Error(err))
			return 0
		}
		val = felt.Zero[felt.Felt]()
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetClassHashAt
func JunoStateGetClassHashAt(readerHandle C.uintptr_t, contractAddress, buffer unsafe.Pointer) C.int {
	context := unwrapContext(readerHandle)

	val, err := context.state.ContractClassHash(makeFeltFromPtr[felt.Address](contractAddress))
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.logger.Error("JunoStateGetClassHashAt failed to read contract class", zap.Error(err))
			return 0
		}
		val = felt.Zero[felt.ClassHash]()
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetCompiledClass
func JunoStateGetCompiledClass(
	readerHandle C.uintptr_t,
	classHash unsafe.Pointer,
	declaredAt *C.uint64_t,
) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	val, err := context.state.Class(makeFeltFromPtr[felt.ClassHash](classHash))
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			context.logger.Error("JunoStateGetCompiledClass failed to read class", zap.Error(err))
		}
		return nil
	}

	compiledClass, err := marshalClassInfo(val.Class)
	if err != nil {
		context.logger.Error("JunoStateGetCompiledClass failed to marshal compiled class", zap.Error(err))
		return nil
	}

	*declaredAt = C.uint64_t(val.At)
	return unsafe.Pointer(cstring(compiledClass))
}

func fillBufferWithFelt[K any](val *felt.Typed[K], buffer unsafe.Pointer) C.int {
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

	val, err := context.state.CompiledClassHash(makeFeltFromPtr[felt.SierraClassHash](classHash))
	if err != nil {
		return 0
	}

	return fillBufferWithFelt(&val, buffer)
}

//export JunoStateGetCompiledClassHashV2
func JunoStateGetCompiledClassHashV2(
	readerHandle C.uintptr_t,
	classHash,
	buffer unsafe.Pointer,
) C.int {
	context := unwrapContext(readerHandle)

	val, err := context.state.CompiledClassHashV2(makeFeltFromPtr[felt.SierraClassHash](classHash))
	if err != nil {
		return 0
	}

	return fillBufferWithFelt(&val, buffer)
}
