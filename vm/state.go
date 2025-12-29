package vm

//#include <stdint.h>
//#include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
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
		// TODO(maksymmalicki): handle errors of both states
		if !errors.Is(err, state.ErrContractNotDeployed) && !errors.Is(err, db.ErrKeyNotFound) {
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
		// TODO(maksymmalicki): handle errors of both states
		if !errors.Is(err, db.ErrKeyNotFound) && !errors.Is(err, state.ErrContractNotDeployed) {
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
		// TODO(maksymmalicki): handle errors of both states
		if !errors.Is(err, db.ErrKeyNotFound) && !errors.Is(err, state.ErrContractNotDeployed) {
			context.log.Errorw("JunoStateGetClassHashAt failed to read contract class", "err", err)
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
			context.log.Errorw("JunoStateGetCompiledClass failed to read class", "err", err)
		}
		return nil
	}

	compiledClass, err := marshalClassInfo(val.Class)
	if err != nil {
		context.log.Errorw("JunoStateGetCompiledClass failed to marshal compiled class", "err", err)
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
