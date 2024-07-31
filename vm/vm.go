package vm

/*
#include <stdint.h>
#include <stdlib.h>
#include <stddef.h>

#define FELT_SIZE 32

typedef struct CallInfo {
	unsigned char contract_address[FELT_SIZE];
	unsigned char class_hash[FELT_SIZE];
	unsigned char entry_point_selector[FELT_SIZE];
	unsigned char** calldata;
	size_t len_calldata;
} CallInfo;

typedef struct BlockInfo {
	unsigned long long block_number;
	unsigned long long block_timestamp;
	unsigned char sequencer_address[FELT_SIZE];
	unsigned char gas_price_wei[FELT_SIZE];
	unsigned char gas_price_fri[FELT_SIZE];
	char* version;
	unsigned char block_hash_to_be_revealed[FELT_SIZE];
	unsigned char data_gas_price_wei[FELT_SIZE];
	unsigned char data_gas_price_fri[FELT_SIZE];
	unsigned char use_blob_data;
} BlockInfo;

extern void cairoVMCall(CallInfo* call_info_ptr, BlockInfo* block_info_ptr, uintptr_t readerHandle, char* chain_id,
	unsigned long long max_steps);

extern void cairoVMExecute(char* txns_json, char* classes_json, char* paid_fees_on_l1_json,
					BlockInfo* block_info_ptr, uintptr_t readerHandle,  char* chain_id,
					unsigned char skip_charge_fee, unsigned char skip_validate, unsigned char err_on_revert);

extern char* setVersionedConstants(char* json);
extern void freeString(char* str);

#cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_rs
#cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_rs
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/cgo"
	"unsafe"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

//go:generate mockgen -destination=../mocks/mock_vm.go -package=mocks github.com/NethermindEth/juno/vm VM
type VM interface {
	Call(callInfo *CallInfo, blockInfo *BlockInfo, state core.StateReader, network *utils.Network, maxSteps uint64, useBlobData bool) ([]*felt.Felt, error) //nolint:lll
	Execute(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt, blockInfo *BlockInfo,
		state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert, useBlobData bool,
	) ([]*felt.Felt, []*felt.Felt, []TransactionTrace, error)
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
	actualFees      []*felt.Felt
	traces          []json.RawMessage
	dataGasConsumed []*felt.Felt
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

//export JunoAppendDataGasConsumed
func JunoAppendDataGasConsumed(readerHandle C.uintptr_t, ptr unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.dataGasConsumed = append(context.dataGasConsumed, makeFeltFromPtr(ptr))
}

func makeFeltFromPtr(ptr unsafe.Pointer) *felt.Felt {
	return new(felt.Felt).SetBytes(C.GoBytes(ptr, felt.Bytes))
}

func makePtrFromFelt(val *felt.Felt) unsafe.Pointer {
	feltBytes := val.Bytes()
	//nolint:gocritic
	return C.CBytes(feltBytes[:])
}

type CallInfo struct {
	ContractAddress *felt.Felt
	ClassHash       *felt.Felt
	Selector        *felt.Felt
	Calldata        []felt.Felt
}

type BlockInfo struct {
	Header                *core.Header
	BlockHashToBeRevealed *felt.Felt
}

func copyFeltIntoCArray(f *felt.Felt, cArrPtr *C.uchar) {
	if f == nil {
		return
	}

	feltBytes := f.Bytes()
	cArr := unsafe.Slice(cArrPtr, len(feltBytes))
	for index := range feltBytes {
		cArr[index] = C.uchar(feltBytes[index])
	}
}

func makeCCallInfo(callInfo *CallInfo) (C.CallInfo, runtime.Pinner) {
	var cCallInfo C.CallInfo
	var pinner runtime.Pinner

	copyFeltIntoCArray(callInfo.ContractAddress, &cCallInfo.contract_address[0])
	copyFeltIntoCArray(callInfo.ClassHash, &cCallInfo.class_hash[0])
	copyFeltIntoCArray(callInfo.Selector, &cCallInfo.entry_point_selector[0])

	if len(callInfo.Calldata) > 0 {
		// prepare calldata in Go heap.
		cCallInfo.len_calldata = C.ulong(len(callInfo.Calldata))
		calldataPtrs := make([]*C.uchar, 0, len(callInfo.Calldata))
		for _, data := range callInfo.Calldata {
			cArr := make([]C.uchar, felt.Bytes)
			copyFeltIntoCArray(&data, &cArr[0])
			pinner.Pin(&cArr[0])
			calldataPtrs = append(calldataPtrs, &cArr[0])
		}
		pinner.Pin(&calldataPtrs[0])
		cCallInfo.calldata = &calldataPtrs[0]
	}
	return cCallInfo, pinner
}

func makeCBlockInfo(blockInfo *BlockInfo, useBlobData bool) C.BlockInfo {
	var cBlockInfo C.BlockInfo

	cBlockInfo.block_number = C.ulonglong(blockInfo.Header.Number)
	cBlockInfo.block_timestamp = C.ulonglong(blockInfo.Header.Timestamp)
	copyFeltIntoCArray(blockInfo.Header.SequencerAddress, &cBlockInfo.sequencer_address[0])
	copyFeltIntoCArray(blockInfo.Header.GasPrice, &cBlockInfo.gas_price_wei[0])
	copyFeltIntoCArray(blockInfo.Header.GasPriceSTRK, &cBlockInfo.gas_price_fri[0])
	cBlockInfo.version = cstring([]byte(blockInfo.Header.ProtocolVersion))
	copyFeltIntoCArray(blockInfo.BlockHashToBeRevealed, &cBlockInfo.block_hash_to_be_revealed[0])
	if blockInfo.Header.L1DAMode == core.Blob {
		copyFeltIntoCArray(blockInfo.Header.L1DataGasPrice.PriceInWei, &cBlockInfo.data_gas_price_wei[0])
		copyFeltIntoCArray(blockInfo.Header.L1DataGasPrice.PriceInFri, &cBlockInfo.data_gas_price_fri[0])
		if useBlobData {
			cBlockInfo.use_blob_data = 1
		} else {
			cBlockInfo.use_blob_data = 0
		}
	}
	return cBlockInfo
}

func (v *vm) Call(callInfo *CallInfo, blockInfo *BlockInfo, state core.StateReader,
	network *utils.Network, maxSteps uint64, useBlobData bool,
) ([]*felt.Felt, error) {
	context := &callContext{
		state:    state,
		response: []*felt.Felt{},
		log:      v.log,
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	C.setVersionedConstants(C.CString("my_json"))

	cCallInfo, callInfoPinner := makeCCallInfo(callInfo)
	cBlockInfo := makeCBlockInfo(blockInfo, useBlobData)
	chainID := C.CString(network.L2ChainID)
	C.cairoVMCall(
		&cCallInfo,
		&cBlockInfo,
		C.uintptr_t(handle),
		chainID,
		C.ulonglong(maxSteps), //nolint:gocritic
	)
	callInfoPinner.Unpin()
	C.free(unsafe.Pointer(chainID))
	C.free(unsafe.Pointer(cBlockInfo.version))

	if context.err != "" {
		return nil, errors.New(context.err)
	}
	return context.response, nil
}

// Execute executes a given transaction set and returns the gas spent per transaction
func (v *vm) Execute(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt,
	blockInfo *BlockInfo, state core.StateReader, network *utils.Network,
	skipChargeFee, skipValidate, errOnRevert, useBlobData bool,
) ([]*felt.Felt, []*felt.Felt, []TransactionTrace, error) {
	context := &callContext{
		state: state,
		log:   v.log,
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	txnsJSON, classesJSON, err := marshalTxnsAndDeclaredClasses(txns, declaredClasses)
	if err != nil {
		return nil, nil, nil, err
	}

	paidFeesOnL1Bytes, err := json.Marshal(paidFeesOnL1)
	if err != nil {
		return nil, nil, nil, err
	}

	paidFeesOnL1CStr := cstring(paidFeesOnL1Bytes)
	txnsJSONCstr := cstring(txnsJSON)
	classesJSONCStr := cstring(classesJSON)

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

	cBlockInfo := makeCBlockInfo(blockInfo, useBlobData)
	chainID := C.CString(network.L2ChainID)
	C.cairoVMExecute(txnsJSONCstr,
		classesJSONCStr,
		paidFeesOnL1CStr,
		&cBlockInfo,
		C.uintptr_t(handle),
		chainID,
		C.uchar(skipChargeFeeByte),
		C.uchar(skipValidateByte),
		C.uchar(errOnRevertByte), //nolint:gocritic
	)

	C.free(unsafe.Pointer(classesJSONCStr))
	C.free(unsafe.Pointer(paidFeesOnL1CStr))
	C.free(unsafe.Pointer(txnsJSONCstr))
	C.free(unsafe.Pointer(chainID))
	C.free(unsafe.Pointer(cBlockInfo.version))

	if context.err != "" {
		if context.errTxnIndex >= 0 {
			return nil, nil, nil, TransactionExecutionError{
				Index: uint64(context.errTxnIndex),
				Cause: errors.New(context.err),
			}
		}
		return nil, nil, nil, errors.New(context.err)
	}

	traces := make([]TransactionTrace, len(context.traces))
	for index, traceJSON := range context.traces {
		if err := json.Unmarshal(traceJSON, &traces[index]); err != nil {
			return nil, nil, nil, fmt.Errorf("unmarshal trace: %v", err)
		}
		//
	}

	return context.actualFees, context.dataGasConsumed, traces, nil
}

func marshalTxnsAndDeclaredClasses(txns []core.Transaction, declaredClasses []core.Class) (json.RawMessage, json.RawMessage, error) { //nolint:lll
	txnJSONs := make([]json.RawMessage, 0, len(txns))
	for _, txn := range txns {
		txnJSON, err := marshalTxn(txn)
		if err != nil {
			return nil, nil, err
		}
		txnJSONs = append(txnJSONs, txnJSON)
	}

	classJSONs := make([]json.RawMessage, 0, len(declaredClasses))
	for _, declaredClass := range declaredClasses {
		declaredClassJSON, cErr := marshalClassInfo(declaredClass)
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

func SetVersionedConstants(filename string) error {
	fd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

	buff, err := io.ReadAll(fd)
	if err != nil {
		return err
	}

	jsonStr := C.CString(string(buff))
	if errCStr := C.setVersionedConstants(jsonStr); errCStr != nil {
		var errStr string = C.GoString(errCStr)
		// empty string is not an error
		if errStr != "" {
			err = errors.New(errStr)
		}
		// here we rely on free call on Rust side, because on Go side we can have different allocator
		C.freeString((*C.char)(unsafe.Pointer(errCStr)))
	}
	C.free(unsafe.Pointer(jsonStr))

	return err
}
