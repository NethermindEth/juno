package vm

//#include <stdint.h>
//#include <stdlib.h>
import "C"

import (
	"errors"
	"fmt"
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
		val = &felt.Zero
	}

	return fillBufferWithFelt(val, buffer)
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
		val = &felt.Zero
	}

	return fillBufferWithFelt(val, buffer)
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
		val = &felt.Zero
	}

	return fillBufferWithFelt(val, buffer)
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

//export JunoStateSetStorage
func JunoStateSetStorage(readerHandle C.uintptr_t, addr, key, value unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)
	addrFelt := makeFeltFromPtr(addr)
	keyFelt := makeFeltFromPtr(key)
	valueFelt := makeFeltFromPtr(value)
	state := context.state.(StateReadWriter)
	if err := state.SetStorage(addrFelt, keyFelt, valueFelt); err != nil { //nolint:gocritic
		return unsafe.Pointer(C.CString(err.Error()))
	}
	return nil
}

//export JunoStateIncrementNonce
func JunoStateIncrementNonce(readerHandle C.uintptr_t, addr unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)
	addrFelt := makeFeltFromPtr(addr)
	state := context.state.(StateReadWriter)
	if err := state.IncrementNonce(addrFelt); err != nil { //nolint:gocritic
		return unsafe.Pointer(C.CString(err.Error()))
	}
	return nil
}

//export JunoStateSetClassHashAt
func JunoStateSetClassHashAt(readerHandle C.uintptr_t, addr, classHash unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)
	addrFelt := makeFeltFromPtr(addr)
	classHashFelt := makeFeltFromPtr(classHash)
	state := context.state.(StateReadWriter)
	if err := state.SetClassHash(addrFelt, classHashFelt); err != nil { //nolint:gocritic
		return unsafe.Pointer(C.CString(err.Error()))
	}
	return nil
}

//export JunoStateSetContractClass
func JunoStateSetContractClass(readerHandle C.uintptr_t, classHash unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)
	classHashFelt := makeFeltFromPtr(classHash)
	class, found := context.declaredClasses[*classHashFelt]
	if !found {
		return unsafe.Pointer(C.CString(fmt.Sprintf("class not declared: %s", classHashFelt)))
	}
	state := context.state.(StateReadWriter)
	if err := state.SetContractClass(classHashFelt, class); err != nil { //nolint:gocritic
		return unsafe.Pointer(C.CString(err.Error()))
	}
	return nil
}

//export JunoStateSetCompiledClassHash
func JunoStateSetCompiledClassHash(readerHandle C.uintptr_t, classHash, compiledClassHash unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)
	classHashFelt := makeFeltFromPtr(classHash)
	compiledClassHashFelt := makeFeltFromPtr(compiledClassHash)
	state := context.state.(StateReadWriter)
	if err := state.SetCompiledClassHash(classHashFelt, compiledClassHashFelt); err != nil {
		return unsafe.Pointer(C.CString(err.Error()))
	}
	return nil
}

func fillBufferWithFelt(val *felt.Felt, buffer unsafe.Pointer) C.int {
	feltBytes := val.Bytes()
	return C.int(copy(unsafe.Slice((*byte)(buffer), felt.Bytes), feltBytes[:]))
}
