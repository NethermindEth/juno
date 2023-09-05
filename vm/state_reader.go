package vm

//#include <stdint.h>
//#include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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
		if !errors.Is(err, core.ErrContractNotDeployed) {
			context.log.Errorw("JunoStateGetStorageAt failed to read contract storage", "err", err)
			return nil
		}
		val = &felt.Zero
	}

	return makePtrFromFelt(val)
}

//export JunoStateGetNonceAt
func JunoStateGetNonceAt(readerHandle C.uintptr_t, contractAddress unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	val, err := context.state.ContractNonce(contractAddressFelt)
	if err != nil {
		if !errors.Is(err, core.ErrContractNotDeployed) {
			context.log.Errorw("JunoStateGetNonceAt failed to read contract nonce", "err", err)
			return nil
		}
		val = &felt.Zero
	}

	return makePtrFromFelt(val)
}

//export JunoStateGetClassHashAt
func JunoStateGetClassHashAt(readerHandle C.uintptr_t, contractAddress unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	contractAddressFelt := makeFeltFromPtr(contractAddress)
	val, err := context.state.ContractClassHash(contractAddressFelt)
	if err != nil {
		if !errors.Is(err, core.ErrContractNotDeployed) {
			context.log.Errorw("JunoStateGetClassHashAt failed to read contract class", "err", err)
			return nil
		}
		val = &felt.Zero
	}

	return makePtrFromFelt(val)
}

//export JunoStateGetCompiledClass
func JunoStateGetCompiledClass(readerHandle C.uintptr_t, classHash unsafe.Pointer) unsafe.Pointer {
	context := unwrapContext(readerHandle)

	classHashFelt := makeFeltFromPtr(classHash)
	val, err := context.state.Class(classHashFelt)
	if err != nil {
		context.log.Errorw("JunoStateGetCompiledClass failed to read class", "err", err)
		return nil
	}

	compiledClass, err := marshalCompiledClass(val.Class)
	if err != nil {
		context.log.Errorw("JunoStateGetCompiledClass failed to marshal compiled class", "err", err)
		return nil
	}

	return unsafe.Pointer(C.CString(string(compiledClass)))
}
