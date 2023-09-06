package vm

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
// extern void cairoVMCall(char* contract_address, char* entry_point_selector, char** calldata, size_t len_calldata,
//					uintptr_t readerHandle, unsigned long long block_number, unsigned long long block_timestamp,
//					char* chain_id);
//
// extern void cairoVMExecute(char* txns_json, char* classes_json, uintptr_t readerHandle, unsigned long long block_number,
//					unsigned long long block_timestamp, char* chain_id, char* sequencer_address, char* paid_fees_on_l1_json,
//					unsigned char skip_charge_fee, char* gas_price);
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

//go:generate mockgen -destination=../mocks/mock_vm.go -package=mocks github.com/NethermindEth/juno/vm VM
type VM interface {
	Call(contractAddr, selector *felt.Felt, calldata []felt.Felt, blockNumber,
		blockTimestamp uint64, state core.StateReader, network utils.Network,
	) ([]*felt.Felt, error)
	Execute(txns []core.Transaction, declaredClasses []core.Class, blockNumber, blockTimestamp uint64,
		sequencerAddress *felt.Felt, state core.StateReader, network utils.Network, paidFeesOnL1 []*felt.Felt,
		skipChargeFee bool, gasPrice *felt.Felt,
	) ([]*felt.Felt, []json.RawMessage, error)
}

type vm struct {
	log utils.SimpleLogger
}

func New(log utils.SimpleLogger) VM {
	return &vm{
		log: log,
	}
}

// callContext manages the context that a Call instance executes on
type callContext struct {
	// state that the call is running on
	state core.StateReader
	log   utils.SimpleLogger
	// err field to be possibly populated in case of an error in execution
	err string
	// response from the executed Cairo function
	response []*felt.Felt
	// fee amount taken per transaction during VM execution
	actualFees []*felt.Felt
	traces     []json.RawMessage
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

//export JunoAppendTrace
func JunoAppendTrace(readerHandle C.uintptr_t, jsonBytes *C.void, bytesLen C.size_t) {
	context := unwrapContext(readerHandle)
	byteSlice := C.GoBytes(unsafe.Pointer(jsonBytes), C.int(bytesLen))
	context.traces = append(context.traces, json.RawMessage(byteSlice))
}

//export JunoAppendResponse
func JunoAppendResponse(readerHandle C.uintptr_t, ptr unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.response = append(context.response, makeFeltFromPtr(ptr))
}

//export JunoAppendActualFee
func JunoAppendActualFee(readerHandle C.uintptr_t, ptr unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.actualFees = append(context.actualFees, makeFeltFromPtr(ptr))
}

func makeFeltFromPtr(ptr unsafe.Pointer) *felt.Felt {
	return new(felt.Felt).SetBytes(C.GoBytes(ptr, felt.Bytes))
}

func makePtrFromFelt(val *felt.Felt) unsafe.Pointer {
	feltBytes := val.Bytes()
	//nolint:gocritic
	return C.CBytes(feltBytes[:])
}

func (v *vm) Call(contractAddr, selector *felt.Felt, calldata []felt.Felt, blockNumber,
	blockTimestamp uint64, state core.StateReader, network utils.Network,
) ([]*felt.Felt, error) {
	context := &callContext{
		state:    state,
		response: []*felt.Felt{},
		log:      v.log,
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	addrBytes := contractAddr.Bytes()
	selectorBytes := selector.Bytes()
	calldataPtrs := []*C.char{}
	for _, data := range calldata {
		bytes := data.Bytes()
		//nolint:gocritic
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
func (v *vm) Execute(txns []core.Transaction, declaredClasses []core.Class, blockNumber, blockTimestamp uint64,
	sequencerAddress *felt.Felt, state core.StateReader, network utils.Network, paidFeesOnL1 []*felt.Felt,
	skipChargeFee bool, gasPrice *felt.Felt,
) ([]*felt.Felt, []json.RawMessage, error) {
	context := &callContext{
		state: state,
		log:   v.log,
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	txnsJSON, classesJSON, err := marshalTxnsAndDeclaredClasses(txns, declaredClasses)
	if err != nil {
		return nil, nil, err
	}

	paidFeesOnL1Bytes, err := json.Marshal(paidFeesOnL1)
	if err != nil {
		return nil, nil, err
	}

	paidFeesOnL1CStr := C.CString(string(paidFeesOnL1Bytes))
	txnsJSONCstr := C.CString(string(txnsJSON))
	classesJSONCStr := C.CString(string(classesJSON))

	sequencerAddressBytes := sequencerAddress.Bytes()
	gasPriceBytes := gasPrice.Bytes()

	var skipChargeFeeByte byte
	if skipChargeFee {
		skipChargeFeeByte = 1
	}

	chainID := C.CString(network.ChainIDString())
	C.cairoVMExecute(txnsJSONCstr,
		classesJSONCStr,
		C.uintptr_t(handle),
		C.ulonglong(blockNumber),
		C.ulonglong(blockTimestamp),
		chainID,
		(*C.char)(unsafe.Pointer(&sequencerAddressBytes[0])),
		paidFeesOnL1CStr,
		C.uchar(skipChargeFeeByte),
		(*C.char)(unsafe.Pointer(&gasPriceBytes[0])),
	)

	C.free(unsafe.Pointer(classesJSONCStr))
	C.free(unsafe.Pointer(paidFeesOnL1CStr))
	C.free(unsafe.Pointer(txnsJSONCstr))
	C.free(unsafe.Pointer(chainID))

	if len(context.err) > 0 {
		return nil, nil, errors.New(context.err)
	}

	return context.actualFees, context.traces, nil
}

func marshalTxnsAndDeclaredClasses(txns []core.Transaction, declaredClasses []core.Class) (json.RawMessage, json.RawMessage, error) {
	txnJSONs := []json.RawMessage{}
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
