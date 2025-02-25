package rpcv8

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var traceFallbackVersion = semver.MustParse("0.13.1")

const excludedVersion = "0.13.1.1"

type TransactionTrace struct {
	Type                  TransactionType     `json:"type,omitempty"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation,omitempty"`
	StateDiff             *vm.StateDiff       `json:"state_diff,omitempty"`
	ExecutionResources    *ExecutionResources `json:"execution_resources,omitempty"`
}

type FunctionInvocation struct {
	ContractAddress    felt.Felt                 `json:"contract_address"`
	EntryPointSelector *felt.Felt                `json:"entry_point_selector,omitempty"`
	Calldata           []felt.Felt               `json:"calldata"`
	CallerAddress      felt.Felt                 `json:"caller_address"`
	ClassHash          *felt.Felt                `json:"class_hash,omitempty"`
	EntryPointType     string                    `json:"entry_point_type,omitempty"`
	CallType           string                    `json:"call_type,omitempty"`
	Result             []felt.Felt               `json:"result"`
	Calls              []FunctionInvocation      `json:"calls"`
	Events             []vm.OrderedEvent         `json:"events"`
	Messages           []vm.OrderedL2toL1Message `json:"messages"`
	ExecutionResources *InnerExecutionResources  `json:"execution_resources,omitempty"`
}

type ExecuteInvocation struct {
	RevertReason        string `json:"revert_reason"`
	*FunctionInvocation `json:",omitempty"`
}

func (e ExecuteInvocation) MarshalJSON() ([]byte, error) {
	if e.FunctionInvocation != nil {
		return json.Marshal(e.FunctionInvocation)
	}
	type alias ExecuteInvocation
	return json.Marshal(alias(e))
}

type InnerExecutionResources struct {
	L1Gas uint64 `json:"l1_gas"`
	L2Gas uint64 `json:"l2_gas"`
}

func adaptBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}
	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}
	traces := make([]TracedBlockTransaction, 0, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]
		trace := TransactionTrace{}
		trace.Type = block.Transactions[index].Type

		trace.FeeTransferInvocation = functionInvocationFromSN(feederTrace.FeeTransferInvocation)
		trace.ValidateInvocation = functionInvocationFromSN(feederTrace.ValidateInvocation)

		fnInvocation := functionInvocationFromSN(feederTrace.FunctionInvocation)
		switch block.Transactions[index].Type {
		case TxnDeploy:
			trace.ConstructorInvocation = fnInvocation
		case TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.ExecuteInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.ExecuteInvocation.FunctionInvocation = fnInvocation
			}
		case TxnL1Handler:
			trace.FunctionInvocation = fnInvocation
		}

		traces = append(traces, TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		})
	}
	return traces, nil
}

func functionInvocationFromSN(snFnInvocation *starknet.FunctionInvocation) *FunctionInvocation {
	if snFnInvocation == nil {
		return nil
	}

	fnInvocation := FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              make([]FunctionInvocation, 0, len(snFnInvocation.InternalCalls)),
		Events:             make([]vm.OrderedEvent, 0, len(snFnInvocation.Events)),
		Messages:           make([]vm.OrderedL2toL1Message, 0, len(snFnInvocation.Messages)),
		ExecutionResources: innerExecutionResourcesFromSN(&snFnInvocation.ExecutionResources),
	}

	for index := range snFnInvocation.InternalCalls {
		fnInvocation.Calls = append(fnInvocation.Calls, *functionInvocationFromSN(&snFnInvocation.InternalCalls[index]))
	}
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]
		fnInvocation.Events = append(fnInvocation.Events, vm.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.Ptr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.Ptr[felt.Felt]),
		})
	}
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]
		fnInvocation.Messages = append(fnInvocation.Messages, vm.OrderedL2toL1Message{
			Order:   snMessage.Order,
			Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
			To:      snMessage.ToAddr,
		})
	}

	return &fnInvocation
}

func innerExecutionResourcesFromSN(resources *starknet.ExecutionResources) *InnerExecutionResources {
	var l1Gas, l2Gas uint64
	if tgc := resources.TotalGasConsumed; tgc != nil {
		l1Gas = tgc.L1Gas
		l2Gas = tgc.L2Gas
	}
	return &InnerExecutionResources{
		L1Gas: l1Gas,
		L2Gas: l2Gas,
	}
}

/****************************************************
		Tracing Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/1ae810e0137cc5d175ace4554892a4f43052be56/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (*TransactionTrace, http.Header, *jsonrpc.Error) {
	return h.traceTransaction(ctx, &hash)
}

func (h *Handler) traceTransaction(ctx context.Context, hash *felt.Felt) (*TransactionTrace, http.Header, *jsonrpc.Error) {
	_, blockHash, _, err := h.bcReader.Receipt(hash)
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, httpHeader, rpccore.ErrTxnHashNotFound
	}

	var block *core.Block
	isPendingBlock := blockHash == nil
	if isPendingBlock {
		var pending *sync.Pending
		pending, err = h.syncReader.Pending()
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, httpHeader, rpccore.ErrTxnHashNotFound
		}
		block = pending.Block
	} else {
		block, err = h.bcReader.BlockByHash(blockHash)
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, httpHeader, rpccore.ErrTxnHashNotFound
		}
	}

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(hash)
	})
	if txIndex == -1 {
		return nil, httpHeader, rpccore.ErrTxnHashNotFound
	}

	traceResults, header, traceBlockErr := h.traceBlockTransactions(ctx, block)
	if traceBlockErr != nil {
		return nil, header, traceBlockErr
	}

	return traceResults[txIndex].TraceRoot, header, nil
}

func (h *Handler) TraceBlockTransactions(ctx context.Context, id BlockID) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		httpHeader := http.Header{}
		httpHeader.Set(ExecutionStepsHeader, "0")
		return nil, httpHeader, rpcErr
	}

	return h.traceBlockTransactions(ctx, block)
}

//nolint:funlen,gocyclo
func (h *Handler) traceBlockTransactions(ctx context.Context, block *core.Block) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	isPending := block.Hash == nil
	if !isPending {
		if blockVer, err := core.ParseBlockVersion(block.ProtocolVersion); err != nil {
			return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
		} else if blockVer.LessThanEqual(traceFallbackVersion) && block.ProtocolVersion != excludedVersion {
			// version <= 0.13.1 and not 0.13.1.1 fetch blocks from feeder gateway
			result, err := h.fetchTraces(ctx, block.Hash)
			if err != nil {
				return nil, httpHeader, err
			}

			txDataAvailability := make(map[felt.Felt]vm.DataAvailability, len(block.Receipts))
			txTotalGasConsumed := make(map[felt.Felt]core.GasConsumed, len(block.Receipts))
			for _, receipt := range block.Receipts {
				if receipt.ExecutionResources == nil {
					continue
				}
				if receiptDA := receipt.ExecutionResources.DataAvailability; receiptDA != nil {
					da := vm.DataAvailability{
						L1Gas:     receiptDA.L1Gas,
						L1DataGas: receiptDA.L1DataGas,
					}
					txDataAvailability[*receipt.TransactionHash] = da
				}
				if receiptTGS := receipt.ExecutionResources.TotalGasConsumed; receiptTGS != nil {
					tgs := core.GasConsumed{
						L1Gas:     receiptTGS.L1Gas,
						L1DataGas: receiptTGS.L1DataGas,
						L2Gas:     receiptTGS.L2Gas,
					}
					txTotalGasConsumed[*receipt.TransactionHash] = tgs
				}
			}

			// add execution resources on root level
			for index, trace := range result {
				// fgw doesn't provide this data in traces endpoint
				// some receipts don't have data availability data in this case we don't
				tgs := txTotalGasConsumed[*trace.TransactionHash]
				result[index].TraceRoot.ExecutionResources = &ExecutionResources{
					L1Gas:     tgs.L1Gas,
					L1DataGas: tgs.L1DataGas,
					L2Gas:     tgs.L2Gas,
				}
			}

			return result, httpHeader, err
		}

		if trace, hit := h.blockTraceCache.Get(rpccore.TraceCacheKey{
			BlockHash: *block.Hash,
		}); hit {
			return trace, httpHeader, nil
		}
	}

	state, closer, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return nil, httpHeader, rpccore.ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in traceBlockTransactions")

	var (
		headState       core.StateReader
		headStateCloser blockchain.StateCloser
	)
	if isPending {
		headState, headStateCloser, err = h.syncReader.PendingState()
	} else {
		headState, headStateCloser, err = h.bcReader.HeadState()
	}
	if err != nil {
		return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	var classes []core.Class
	paidFeesOnL1 := []*felt.Felt{}

	for _, transaction := range block.Transactions {
		switch tx := transaction.(type) {
		case *core.DeclareTransaction:
			class, stateErr := headState.Class(tx.ClassHash)
			if stateErr != nil {
				return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
			}
			classes = append(classes, class.Class)
		case *core.L1HandlerTransaction:
			var fee felt.Felt
			paidFeesOnL1 = append(paidFeesOnL1, fee.SetUint64(1))
		}
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(block.Number)
	if err != nil {
		return nil, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}
	network := h.bcReader.Network()
	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	executionResult, err := h.vm.Execute(block.Transactions, classes, paidFeesOnL1,
		&blockInfo, state, network, false, false, false)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]TracedBlockTransaction, 0, len(executionResult.Traces))
	for index, trace := range executionResult.Traces {
		executionResult.Traces[index].ExecutionResources = &vm.ExecutionResources{
			L1Gas:                executionResult.GasConsumed[index].L1Gas,
			L1DataGas:            executionResult.GasConsumed[index].L1DataGas,
			L2Gas:                executionResult.GasConsumed[index].L2Gas,
			ComputationResources: trace.TotalComputationResources(),
			DataAvailability: &vm.DataAvailability{
				L1Gas:     executionResult.DataAvailability[index].L1Gas,
				L1DataGas: executionResult.DataAvailability[index].L1DataGas,
			},
		}
		result = append(result, TracedBlockTransaction{
			TraceRoot:       AdaptTrace(&executionResult.Traces[index]),
			TransactionHash: block.Transactions[index].Hash(),
		})
	}

	if !isPending {
		h.blockTraceCache.Add(rpccore.TraceCacheKey{
			BlockHash: *block.Hash,
		}, result)
	}

	return result, httpHeader, nil
}

func AdaptTrace(trace *vm.TransactionTrace) *TransactionTrace {
	var executeInvocation *ExecuteInvocation
	if vmExecuteInvocation := trace.ExecuteInvocation; vmExecuteInvocation != nil {
		executeInvocation = &ExecuteInvocation{
			RevertReason:       vmExecuteInvocation.RevertReason,
			FunctionInvocation: functionInvocationFromVM(vmExecuteInvocation.FunctionInvocation),
		}
	}

	return &TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    functionInvocationFromVM(trace.ValidateInvocation),
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: functionInvocationFromVM(trace.FeeTransferInvocation),
		ConstructorInvocation: functionInvocationFromVM(trace.ConstructorInvocation),
		FunctionInvocation:    functionInvocationFromVM(trace.FunctionInvocation),
		StateDiff:             trace.StateDiff,
		ExecutionResources:    executionResourcesFromVM(trace.ExecutionResources),
	}
}

func executionResourcesFromVM(resources *vm.ExecutionResources) *ExecutionResources {
	if resources == nil {
		return nil
	}
	return &ExecutionResources{
		L1Gas:     resources.L1Gas,
		L1DataGas: resources.L1DataGas,
		L2Gas:     resources.L2Gas,
	}
}

func functionInvocationFromVM(vmFnInvocation *vm.FunctionInvocation) *FunctionInvocation {
	if vmFnInvocation == nil {
		return nil
	}
	var calls []FunctionInvocation
	if vmCalls := vmFnInvocation.Calls; vmCalls != nil {
		calls = make([]FunctionInvocation, len(vmCalls))
		for i := range vmCalls {
			calls[i] = *functionInvocationFromVM(&vmCalls[i])
		}
	}
	return &FunctionInvocation{
		ContractAddress:    vmFnInvocation.ContractAddress,
		EntryPointSelector: vmFnInvocation.EntryPointSelector,
		Calldata:           vmFnInvocation.Calldata,
		CallerAddress:      vmFnInvocation.CallerAddress,
		ClassHash:          vmFnInvocation.ClassHash,
		EntryPointType:     vmFnInvocation.EntryPointType,
		CallType:           vmFnInvocation.CallType,
		Result:             vmFnInvocation.Result,
		Calls:              calls,
		Events:             vmFnInvocation.Events,
		Messages:           vmFnInvocation.Messages,
		ExecutionResources: innerExecutionResourcesFromVM(vmFnInvocation.ExecutionResources),
	}
}

func innerExecutionResourcesFromVM(resources *vm.ExecutionResources) *InnerExecutionResources {
	if resources == nil {
		return nil
	}
	return &InnerExecutionResources{
		L1Gas: resources.L1Gas,
		L2Gas: resources.L2Gas,
	}
}

func (h *Handler) fetchTraces(ctx context.Context, blockHash *felt.Felt) ([]TracedBlockTransaction, *jsonrpc.Error) {
	rpcBlock, err := h.BlockWithTxs(BlockID{
		Hash: blockHash, // known non-nil
	})
	if err != nil {
		return nil, err
	}

	if h.feederClient == nil {
		return nil, rpccore.ErrInternal.CloneWithData("no feeder client configured")
	}

	blockTrace, fErr := h.feederClient.BlockTrace(ctx, blockHash.String())
	if fErr != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(fErr.Error())
	}

	traces, aErr := adaptBlockTrace(rpcBlock, blockTrace)
	if aErr != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(aErr.Error())
	}

	return traces, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/e0b76ed0d8d8eba405e182371f9edac8b2bcbc5a/api/starknet_api_openrpc.json#L401-L445
func (h *Handler) Call(funcCall FunctionCall, id BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	state, closer, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_call")

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	classHash, err := state.ContractClassHash(&funcCall.ContractAddress)
	if err != nil {
		return nil, rpccore.ErrContractNotFound
	}

	declaredClass, err := state.Class(classHash)
	if err != nil {
		return nil, rpccore.ErrClassHashNotFound
	}

	var sierraVersion string
	if class, ok := declaredClass.Class.(*core.Cairo1Class); ok {
		sierraVersion = class.SemanticVersion
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	res, err := h.vm.Call(&vm.CallInfo{
		ContractAddress: &funcCall.ContractAddress,
		Selector:        &funcCall.EntryPointSelector,
		Calldata:        funcCall.Calldata,
		ClassHash:       classHash,
	}, &vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}, state, h.bcReader.Network(), h.callMaxSteps, sierraVersion)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, MakeContractError(err)
	}
	if res.ExecutionFailed {
		// the blockifier 0.13.4 update requires us to check if the execution failed,
		// and if so, return ErrEntrypointNotFound if res.Result[0]==EntrypointNotFoundFelt,
		// otherwise we should wrap the result in ErrContractError
		if len(res.Result) != 0 && res.Result[0].String() == rpccore.EntrypointNotFoundFelt {
			return nil, rpccore.ErrEntrypointNotFound
		}
		// Todo: There is currently no standardised way to format these error messages
		return nil, MakeContractError(errors.New(utils.FeltArrToString(res.Result)))
	}
	return res.Result, nil
}
