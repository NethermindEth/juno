package vm

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
// extern void cairoVMCall(char* contract_address, char* entry_point_selector, char** calldata, size_t len_calldata,
//					uintptr_t readerHandle, unsigned long long block_number, unsigned long long block_timestamp,
//					char* chain_id);
//
// extern void cairoVMExecute(char* txns_json, char* classes_json, uintptr_t readerHandle, unsigned long long block_number,
//					unsigned long long block_timestamp, char* chain_id, char* sequencer_address);
//
// #cgo LDFLAGS: -L./rust/target/release -ljuno_starknet_rs -lm -ldl
import "C"

import (
	"encoding/json"
	"errors"
	"runtime/cgo"
	"unsafe"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

// callContext manages the context that a Call instance executes on
type callContext struct {
	// state that the call is running on
	state core.StateReader
	// err field to be possibly populated in case of an error in execution
	err string
	// response from the executed Cairo function
	response []*felt.Felt
	// amount of gas consumed per transaction during VM execution
	gasConsumed []*felt.Felt
}

func unwrapContext(readerHandle C.uintptr_t) *callContext {
	context, ok := cgo.Handle(readerHandle).Value().(*callContext)
	if !ok {
		panic("cannot cast reader")
	}

	return context
}

//export JunoReportError
func JunoReportError(readerHandle C.uintptr_t, str *C.char) {
	context := unwrapContext(readerHandle)
	context.err = C.GoString(str)
}

//export JunoAppendResponse
func JunoAppendResponse(readerHandle C.uintptr_t, ptr unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.response = append(context.response, makeFeltFromPtr(ptr))
}

//export JunoAppendGasConsumed
func JunoAppendGasConsumed(readerHandle C.uintptr_t, ptr unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.gasConsumed = append(context.gasConsumed, makeFeltFromPtr(ptr))
}

func makeFeltFromPtr(ptr unsafe.Pointer) *felt.Felt {
	return new(felt.Felt).SetBytes(C.GoBytes(ptr, felt.Bytes))
}

func makePtrFromFelt(val *felt.Felt) unsafe.Pointer {
	feltBytes := val.Bytes()
	return C.CBytes(feltBytes[:])
}

func Call(contractAddr, selector *felt.Felt, calldata []felt.Felt, blockNumber,
	blockTimestamp uint64, state core.StateReader, network utils.Network,
) ([]*felt.Felt, error) {
	context := &callContext{
		state:    state,
		response: []*felt.Felt{},
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	addrBytes := contractAddr.Bytes()
	selectorBytes := selector.Bytes()
	calldataPtrs := []*C.char{}
	for _, data := range calldata {
		bytes := data.Bytes()
		calldataPtrs = append(calldataPtrs, (*C.char)(C.CBytes(bytes[:])))
	}
	calldataArrPtr := unsafe.Pointer(nil)
	if len(calldataPtrs) > 0 {
		calldataArrPtr = unsafe.Pointer(&calldataPtrs[0])
	}

	chainID := C.CString(network.ChainIDString())
	C.cairoVMCall((*C.char)(unsafe.Pointer(&addrBytes[0])),
		(*C.char)(unsafe.Pointer(&selectorBytes[0])),
		(**C.char)(calldataArrPtr),
		C.size_t(len(calldataPtrs)),
		C.uintptr_t(handle),
		C.ulonglong(blockNumber),
		C.ulonglong(blockTimestamp),
		chainID)

	for _, ptr := range calldataPtrs {
		C.free(unsafe.Pointer(ptr))
	}
	C.free(unsafe.Pointer(chainID))

	if len(context.err) > 0 {
		return nil, errors.New(context.err)
	}
	return context.response, nil
}

// Execute executes a given transaction set and returns the gas spent per transaction
func Execute(txns []core.Transaction, declaredClasses []core.Class, blockNumber, blockTimestamp uint64,
	sequencerAddress *felt.Felt, state core.StateReader, network utils.Network,
) ([]*felt.Felt, error) {
	context := &callContext{
		state: state,
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	txnsJSON, classesJSON, err := marshalTxnsAndDeclaredClasses(txns, declaredClasses)
	if err != nil {
		return nil, err
	}

	txnsJSONCstr := C.CString(string(txnsJSON))
	classesJSONCStr := C.CString(string(classesJSON))

	sequencerAddressBytes := sequencerAddress.Bytes()
	chainID := C.CString(network.ChainIDString())
	C.cairoVMExecute(txnsJSONCstr,
		classesJSONCStr,
		C.uintptr_t(handle),
		C.ulonglong(blockNumber),
		C.ulonglong(blockTimestamp),
		chainID,
		(*C.char)(unsafe.Pointer(&sequencerAddressBytes[0])))

	C.free(unsafe.Pointer(classesJSONCStr))
	C.free(unsafe.Pointer(txnsJSONCstr))
	C.free(unsafe.Pointer(chainID))

	if len(context.err) > 0 {
		return nil, errors.New(context.err)
	}
	return context.gasConsumed, nil
}

func marshalTxnsAndDeclaredClasses(txns []core.Transaction, declaredClasses []core.Class) (json.RawMessage, json.RawMessage, error) {
	var txnJSONs []json.RawMessage
	for _, txn := range txns {
		txnJSON, err := marshalTxn(txn)
		if err != nil {
			return nil, nil, err
		}
		txnJSONs = append(txnJSONs, txnJSON)
	}

	classJSONs := []json.RawMessage{}
	for _, declaredClass := range declaredClasses {
		declaredClassJSON, cErr := marshalDeclaredClass(declaredClass)
		if cErr != nil {
			return nil, nil, cErr
		}
		classJSONs = append(classJSONs, declaredClassJSON)
	}

	txnsJSON, err := json.Marshal(txnJSONs)
	if err != nil {
		return nil, nil, err
	}
	classesJSON, err := json.Marshal(classJSONs)
	if err != nil {
		return nil, nil, err
	}

	return txnsJSON, classesJSON, nil
}
