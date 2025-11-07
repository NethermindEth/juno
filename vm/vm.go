package vm

/*
#cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_rs -lbz2
#cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_rs -lbz2

#include "vm_ffi.h"
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
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

const (
	DefaultMaxSteps = 4_000_000
	DefaultMaxGas   = 100_000_000
)

type ExecutionResults struct {
	OverallFees      []*felt.Felt
	DataAvailability []core.DataAvailability
	GasConsumed      []core.GasConsumed
	Traces           []TransactionTrace
	NumSteps         uint64
	Receipts         []TransactionReceipt
}

type CallResult struct {
	Result          []*felt.Felt
	StateDiff       StateDiff
	ExecutionFailed bool
}

//go:generate mockgen -destination=../mocks/mock_vm.go -package=mocks github.com/NethermindEth/juno/vm VM
type VM interface {
	Call(
		callInfo *CallInfo,
		blockInfo *BlockInfo,
		state core.StateReader,
		maxSteps uint64,
		maxGas uint64,
		structuredErrStack,
		returnStateDiff bool,
	) (CallResult, error)
	Execute(
		txns []core.Transaction,
		declaredClasses []core.ClassDefinition,
		paidFeesOnL1 []*felt.Felt,
		blockInfo *BlockInfo,
		state core.StateReader,
		skipChargeFee,
		skipValidate,
		errOnRevert,
		errStack,
		allowBinarySearch bool,
		isEstimateFee bool,
	) (ExecutionResults, error)
}

type vm struct {
	chainInfo       *ChainInfo
	log             utils.SimpleLogger
	concurrencyMode bool
}

func New(chainInfo *ChainInfo, concurrencyMode bool, log utils.SimpleLogger) VM {
	return &vm{
		chainInfo:       chainInfo,
		log:             log,
		concurrencyMode: concurrencyMode,
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
	stateDiff       []json.RawMessage
	daGas           []core.DataAvailability
	gasConsumed     []core.GasConsumed
	executionSteps  uint64
	receipts        []json.RawMessage
	declaredClasses map[felt.Felt]core.ClassDefinition
	executionFailed bool
}

func unwrapContext(readerHandle C.uintptr_t) *callContext {
	context, ok := cgo.Handle(readerHandle).Value().(*callContext)
	if !ok {
		panic("cannot cast reader")
	}

	return context
}

//export JunoReportError
func JunoReportError(readerHandle C.uintptr_t, txnIndex C.long, str *C.char, executionFailed C.uchar) {
	context := unwrapContext(readerHandle)
	context.errTxnIndex = int64(txnIndex)
	context.err = C.GoString(str)
	context.executionFailed = executionFailed == 1
}

//export JunoAppendReceipt
func JunoAppendReceipt(readerHandle C.uintptr_t, jsonBytes *C.void, bytesLen C.size_t) {
	context := unwrapContext(readerHandle)
	byteSlice := C.GoBytes(unsafe.Pointer(jsonBytes), C.int(bytesLen))
	context.receipts = append(context.receipts, json.RawMessage(byteSlice))
}

//export JunoAppendTrace
func JunoAppendTrace(readerHandle C.uintptr_t, jsonBytes *C.void, bytesLen C.size_t) {
	context := unwrapContext(readerHandle)
	byteSlice := C.GoBytes(unsafe.Pointer(jsonBytes), C.int(bytesLen))
	context.traces = append(context.traces, json.RawMessage(byteSlice))
}

//export JunoAppendStateDiff
func JunoAppendStateDiff(readerHandle C.uintptr_t, jsonBytes *C.void, bytesLen C.size_t) {
	context := unwrapContext(readerHandle)
	byteSlice := C.GoBytes(unsafe.Pointer(jsonBytes), C.int(bytesLen))
	context.stateDiff = append(context.stateDiff, json.RawMessage(byteSlice))
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

//export JunoAppendDAGas
func JunoAppendDAGas(readerHandle C.uintptr_t, ptr, ptr2 unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.daGas = append(context.daGas, core.DataAvailability{
		L1Gas:     makeFeltFromPtr(ptr).Uint64(),
		L1DataGas: makeFeltFromPtr(ptr2).Uint64(),
	})
}

//export JunoAppendGasConsumed
func JunoAppendGasConsumed(readerHandle C.uintptr_t, ptr, ptr2, ptr3 unsafe.Pointer) {
	context := unwrapContext(readerHandle)
	context.gasConsumed = append(context.gasConsumed, core.GasConsumed{
		L1Gas:     makeFeltFromPtr(ptr).Uint64(),
		L1DataGas: makeFeltFromPtr(ptr2).Uint64(),
		L2Gas:     makeFeltFromPtr(ptr3).Uint64(),
	})
}

//export JunoAddExecutionSteps
func JunoAddExecutionSteps(readerHandle C.uintptr_t, execSteps C.ulonglong) {
	context := unwrapContext(readerHandle)
	context.executionSteps += uint64(execSteps)
}

func makeFeltFromPtr(ptr unsafe.Pointer) *felt.Felt {
	return new(felt.Felt).SetBytes(C.GoBytes(ptr, felt.Bytes))
}

type CallInfo struct {
	ContractAddress *felt.Felt
	ClassHash       *felt.Felt
	Selector        *felt.Felt
	Calldata        []felt.Felt
}

type ChainInfo struct {
	ChainID           string
	FeeTokenAddresses starknet.FeeTokenAddresses
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

func makeCChainInfo(chainInfo *ChainInfo) C.ChainInfo {
	var cChainInfo C.ChainInfo

	cChainInfo.chain_id = C.CString(chainInfo.ChainID)
	copyFeltIntoCArray(
		&chainInfo.FeeTokenAddresses.EthL2TokenAddress,
		&cChainInfo.eth_fee_token_address[0],
	)
	copyFeltIntoCArray(
		&chainInfo.FeeTokenAddresses.StrkL2TokenAddress,
		&cChainInfo.strk_fee_token_address[0],
	)

	return cChainInfo
}

func makeCBlockInfo(blockInfo *BlockInfo) C.BlockInfo {
	var cBlockInfo C.BlockInfo

	cBlockInfo.block_number = C.ulonglong(blockInfo.Header.Number)
	cBlockInfo.block_timestamp = C.ulonglong(blockInfo.Header.Timestamp)
	cBlockInfo.is_pending = toUchar(blockInfo.Header.Hash == nil)
	copyFeltIntoCArray(blockInfo.Header.SequencerAddress, &cBlockInfo.sequencer_address[0])
	copyFeltIntoCArray(blockInfo.Header.L1GasPriceETH, &cBlockInfo.l1_gas_price_wei[0])
	copyFeltIntoCArray(blockInfo.Header.L1GasPriceSTRK, &cBlockInfo.l1_gas_price_fri[0])
	cBlockInfo.version = cstring([]byte(blockInfo.Header.ProtocolVersion))
	copyFeltIntoCArray(blockInfo.BlockHashToBeRevealed, &cBlockInfo.block_hash_to_be_revealed[0])
	if blockInfo.Header.L1DAMode == core.Blob {
		copyFeltIntoCArray(blockInfo.Header.L1DataGasPrice.PriceInWei, &cBlockInfo.l1_data_gas_price_wei[0])
		copyFeltIntoCArray(blockInfo.Header.L1DataGasPrice.PriceInFri, &cBlockInfo.l1_data_gas_price_fri[0])
		cBlockInfo.use_blob_data = 1
	}
	if blockInfo.Header.L2GasPrice != nil {
		copyFeltIntoCArray(blockInfo.Header.L2GasPrice.PriceInWei, &cBlockInfo.l2_gas_price_wei[0])
		copyFeltIntoCArray(blockInfo.Header.L2GasPrice.PriceInFri, &cBlockInfo.l2_gas_price_fri[0])
	}
	return cBlockInfo
}

func (v *vm) Call(
	callInfo *CallInfo,
	blockInfo *BlockInfo,
	state core.StateReader,
	maxSteps uint64,
	maxGas uint64,
	structuredErrStack,
	returnStateDiff bool,
) (CallResult, error) {
	context := &callContext{
		state:    state,
		response: []*felt.Felt{},
		log:      v.log,
	}

	handle := cgo.NewHandle(context)
	defer handle.Delete()

	cCallInfo, callInfoPinner := makeCCallInfo(callInfo)
	cBlockInfo := makeCBlockInfo(blockInfo)
	cChainInfo := makeCChainInfo(v.chainInfo)
	// TODO: set initial_gas as maxGas in the next PR
	C.cairoVMCall(
		&cCallInfo,
		&cBlockInfo,
		&cChainInfo,
		C.uintptr_t(handle),
		C.ulonglong(maxSteps),
		C.ulonglong(maxGas),
		toUchar(v.concurrencyMode),
		toUchar(structuredErrStack), //nolint:gocritic // See https://github.com/go-critic/go-critic/issues/897
		toUchar(returnStateDiff),    //nolint:gocritic
	)
	callInfoPinner.Unpin()
	C.free(unsafe.Pointer(cChainInfo.chain_id))
	C.free(unsafe.Pointer(cBlockInfo.version))

	if context.err != "" && !context.executionFailed {
		return CallResult{}, errors.New(context.err)
	}

	stateDiff := StateDiff{}
	if returnStateDiff {
		for _, statediffJSON := range context.stateDiff {
			err := json.Unmarshal(statediffJSON, &stateDiff)
			if err != nil {
				return CallResult{}, fmt.Errorf("unmarshal state diff: %v", err)
			}
		}
	}
	return CallResult{Result: context.response, StateDiff: stateDiff, ExecutionFailed: context.executionFailed}, nil
}

// Execute executes a given transaction set and returns the gas spent per transaction
func (v *vm) Execute(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
	paidFeesOnL1 []*felt.Felt,
	blockInfo *BlockInfo,
	state core.StateReader,
	skipChargeFee,
	skipValidate,
	errOnRevert,
	errorStack,
	allowBinarySearch bool,
	isEstimateFee bool,
) (ExecutionResults, error) {
	context := &callContext{
		state: state,
		log:   v.log,
	}
	handle := cgo.NewHandle(context)
	defer handle.Delete()

	txnsJSON, classesJSON, err := marshalTxnsAndDeclaredClasses(txns, declaredClasses)
	if err != nil {
		return ExecutionResults{}, err
	}

	paidFeesOnL1Bytes, err := json.Marshal(paidFeesOnL1)
	if err != nil {
		return ExecutionResults{}, err
	}

	paidFeesOnL1CStr := cstring(paidFeesOnL1Bytes)
	txnsJSONCstr := cstring(txnsJSON)
	classesJSONCStr := cstring(classesJSON)

	cBlockInfo := makeCBlockInfo(blockInfo)
	cChainInfo := makeCChainInfo(v.chainInfo)
	C.cairoVMExecute(txnsJSONCstr,
		classesJSONCStr,
		paidFeesOnL1CStr,
		&cBlockInfo,
		&cChainInfo,
		C.uintptr_t(handle),
		toUchar(skipChargeFee),
		toUchar(skipValidate),
		toUchar(errOnRevert),
		toUchar(v.concurrencyMode),
		toUchar(errorStack),
		toUchar(allowBinarySearch), //nolint:gocritic // See https://github.com/go-critic/go-critic/issues/897
		toUchar(isEstimateFee),
	)

	C.free(unsafe.Pointer(classesJSONCStr))
	C.free(unsafe.Pointer(paidFeesOnL1CStr))
	C.free(unsafe.Pointer(txnsJSONCstr))
	C.free(unsafe.Pointer(cChainInfo.chain_id))
	C.free(unsafe.Pointer(cBlockInfo.version))

	if context.err != "" {
		if context.errTxnIndex >= 0 {
			return ExecutionResults{}, TransactionExecutionError{
				Index: uint64(context.errTxnIndex),
				Cause: json.RawMessage(context.err),
			}
		}
		return ExecutionResults{}, errors.New(context.err)
	}

	traces := make([]TransactionTrace, len(context.traces))
	for index, traceJSON := range context.traces {
		if err := json.Unmarshal(traceJSON, &traces[index]); err != nil {
			return ExecutionResults{}, fmt.Errorf("unmarshal trace: %v", err)
		}
	}
	receipts := make([]TransactionReceipt, len(context.receipts))
	for index, traceJSON := range context.receipts {
		if err := json.Unmarshal(traceJSON, &receipts[index]); err != nil {
			return ExecutionResults{}, fmt.Errorf("unmarshal receipt: %v", err)
		}
	}
	return ExecutionResults{
		OverallFees:      context.actualFees,
		DataAvailability: context.daGas,
		GasConsumed:      context.gasConsumed,
		Traces:           traces,
		NumSteps:         context.executionSteps,
		Receipts:         receipts,
	}, nil
}

func marshalTxnsAndDeclaredClasses(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
) (json.RawMessage, json.RawMessage, error) {
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

func toUchar(b bool) C.uchar {
	if b {
		return 1
	}
	return 0
}
