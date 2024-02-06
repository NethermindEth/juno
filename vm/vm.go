package vm

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
// extern void cairoVMCall(char* contract_address, char* class_hash, char* entry_point_selector, char** calldata,
//					 size_t len_calldata, uintptr_t readerHandle, unsigned long long block_number,
//					 unsigned long long block_timestamp, char* chain_id, unsigned long long max_steps);
//
// extern void cairoVMExecute(char* txns_json, char* classes_json, uintptr_t readerHandle, unsigned long long block_number,
//					unsigned long long block_timestamp, char* chain_id, char* sequencer_address, char* paid_fees_on_l1_json,
//					unsigned char skip_charge_fee, unsigned char skip_validate, unsigned char err_on_revert, char* gas_price_wei,
//					char* gas_price_strk, unsigned char legacy_json);
//
// #cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_rs -ldl -lm
// #cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_rs -ldl -lm
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime/cgo"
	"unsafe"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

//go:generate mockgen -destination=../mocks/mock_vm.go -package=mocks github.com/NethermindEth/juno/vm VM
type VM interface {
	Call(contractAddr, classHash, selector *felt.Felt, calldata []felt.Felt, blockNumber,
		blockTimestamp uint64, state core.StateReader, network *utils.Network, maxSteps uint64,
	) ([]*felt.Felt, error)
	Execute(txns []core.Transaction, declaredClasses []core.Class, blockNumber, blockTimestamp uint64,
		sequencerAddress *felt.Felt, state core.StateReader, network *utils.Network, paidFeesOnL1 []*felt.Felt,
		skipChargeFee, skipValidate, errOnRevert bool, gasPriceWEI *felt.Felt, gasPriceSTRK *felt.Felt, legacyTraceJSON bool,
	) ([]*felt.Felt, []TransactionTrace, error)
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
	// index of the transaction that generated err
	errTxnIndex int64
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
func JunoReportError(readerHandle C.uintptr_t, txnIndex C.long, str *C.char) {
	context := unwrapContext(readerHandle)
	context.errTxnIndex = int64(txnIndex)
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

func (v *vm) Call(contractAddr, classHash, selector *felt.Felt, calldata []felt.Felt, blockNumber,
	blockTimestamp uint64, state core.StateReader, network *utils.Network, maxSteps uint64,
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

	classHashPtr := (*byte)(nil)
	if classHash != nil {
		classHashBytes := classHash.Bytes()
		classHashPtr = &classHashBytes[0]
	}
	chainID := C.CString(network.L2ChainID)
	C.cairoVMCall((*C.char)(unsafe.Pointer(&addrBytes[0])),
		(*C.char)(unsafe.Pointer(classHashPtr)),
		(*C.char)(unsafe.Pointer(&selectorBytes[0])),
		(**C.char)(calldataArrPtr),
		C.size_t(len(calldataPtrs)),
		C.uintptr_t(handle),
		C.ulonglong(blockNumber),
		C.ulonglong(blockTimestamp),
		chainID,
		C.ulonglong(maxSteps),
	)

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
	sequencerAddress *felt.Felt, state core.StateReader, network *utils.Network, paidFeesOnL1 []*felt.Felt,
	skipChargeFee, skipValidate, errOnRevert bool, gasPriceWEI *felt.Felt, gasPriceSTRK *felt.Felt, legacyTraceJSON bool,
) ([]*felt.Felt, []TransactionTrace, error) {
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

	paidFeesOnL1CStr := cstring(paidFeesOnL1Bytes)
	txnsJSONCstr := cstring(txnsJSON)
	classesJSONCStr := cstring(classesJSON)

	sequencerAddressBytes := sequencerAddress.Bytes()
	gasPriceWEIBytes := gasPriceWEI.Bytes()

	if gasPriceSTRK == nil {
		gasPriceSTRK = &felt.Zero
	}
	gasPriceSTRKBytes := gasPriceSTRK.Bytes()

	var skipChargeFeeByte byte
	if skipChargeFee {
		skipChargeFeeByte = 1
	}

	var skipValidateByte byte
	if skipValidate {
		skipValidateByte = 1
	}

	var errOnRevertByte byte
	if errOnRevert {
		errOnRevertByte = 1
	}
	var legacyTraceJSONByte byte
	if legacyTraceJSON {
		legacyTraceJSONByte = 1
	}

	chainID := C.CString(network.L2ChainID)
	C.cairoVMExecute(txnsJSONCstr,
		classesJSONCStr,
		C.uintptr_t(handle),
		C.ulonglong(blockNumber),
		C.ulonglong(blockTimestamp),
		chainID,
		(*C.char)(unsafe.Pointer(&sequencerAddressBytes[0])),
		paidFeesOnL1CStr,
		C.uchar(skipChargeFeeByte),
		C.uchar(skipValidateByte),
		C.uchar(errOnRevertByte),
		(*C.char)(unsafe.Pointer(&gasPriceWEIBytes[0])),
		(*C.char)(unsafe.Pointer(&gasPriceSTRKBytes[0])),
		C.uchar(legacyTraceJSONByte),
	)

	C.free(unsafe.Pointer(classesJSONCStr))
	C.free(unsafe.Pointer(paidFeesOnL1CStr))
	C.free(unsafe.Pointer(txnsJSONCstr))
	C.free(unsafe.Pointer(chainID))

	if len(context.err) > 0 {
		if context.errTxnIndex >= 0 {
			return nil, nil, TransactionExecutionError{
				Index: uint64(context.errTxnIndex),
				Cause: errors.New(context.err),
			}
		}
		return nil, nil, errors.New(context.err)
	}

	traces := make([]TransactionTrace, len(context.traces))
	for index, traceJSON := range context.traces {
		if err := json.Unmarshal(traceJSON, &traces[index]); err != nil {
			return nil, nil, fmt.Errorf("unmarshal trace: %v", err)
		}
	}

	return context.actualFees, traces, nil
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
		declaredClassJSON, cErr := marshalCompiledClass(declaredClass)
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
