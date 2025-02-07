package rpcv7

import (
	"context"
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
		trace := vm.TransactionTrace{}
		trace.Type = vm.TransactionType(block.Transactions[index].Type)

		trace.FeeTransferInvocation = adaptFunctionInvocation(feederTrace.FeeTransferInvocation)
		trace.ValidateInvocation = adaptFunctionInvocation(feederTrace.ValidateInvocation)

		fnInvocation := adaptFunctionInvocation(feederTrace.FunctionInvocation)
		switch block.Transactions[index].Type {
		case TxnDeploy:
			trace.ConstructorInvocation = fnInvocation
		case TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(vm.ExecuteInvocation)
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

func adaptFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) *vm.FunctionInvocation {
	if snFnInvocation == nil {
		return nil
	}

	fnInvocation := vm.FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              make([]vm.FunctionInvocation, 0, len(snFnInvocation.InternalCalls)),
		Events:             make([]vm.OrderedEvent, 0, len(snFnInvocation.Events)),
		Messages:           make([]vm.OrderedL2toL1Message, 0, len(snFnInvocation.Messages)),
		ExecutionResources: adaptFeederExecutionResources(&snFnInvocation.ExecutionResources),
	}

	for index := range snFnInvocation.InternalCalls {
		fnInvocation.Calls = append(fnInvocation.Calls, *adaptFunctionInvocation(&snFnInvocation.InternalCalls[index]))
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

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) *vm.ExecutionResources {
	builtins := &resources.BuiltinInstanceCounter
	var l1Gas, l1DataGas, l2Gas uint64
	if tgs := resources.TotalGasConsumed; tgs != nil {
		l1Gas = tgs.L1Gas
		l1DataGas = tgs.L1DataGas
		l2Gas = tgs.L2Gas
	}
	return &vm.ExecutionResources{
		L1Gas:     l1Gas,
		L1DataGas: l1DataGas,
		L2Gas:     l2Gas,
		ComputationResources: vm.ComputationResources{
			Steps:        resources.Steps,
			MemoryHoles:  resources.MemoryHoles,
			Pedersen:     builtins.Pedersen,
			RangeCheck:   builtins.RangeCheck,
			Bitwise:      builtins.Bitwise,
			Ecdsa:        builtins.Ecsda,
			EcOp:         builtins.EcOp,
			Keccak:       builtins.Keccak,
			Poseidon:     builtins.Poseidon,
			SegmentArena: builtins.SegmentArena,
		},
	}
}

/****************************************************
		Tracing Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/1ae810e0137cc5d175ace4554892a4f43052be56/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (*vm.TransactionTrace, http.Header, *jsonrpc.Error) {
	return h.traceTransaction(ctx, &hash)
}

func (h *Handler) traceTransaction(ctx context.Context, hash *felt.Felt) (*vm.TransactionTrace, http.Header, *jsonrpc.Error) {
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
				da := txDataAvailability[*trace.TransactionHash]
				tgs := txTotalGasConsumed[*trace.TransactionHash]
				result[index].TraceRoot.ExecutionResources = &vm.ExecutionResources{
					L1Gas:                tgs.L1Gas,
					L1DataGas:            tgs.L1DataGas,
					L2Gas:                tgs.L2Gas,
					ComputationResources: trace.TraceRoot.TotalComputationResources(),
					DataAvailability:     &da,
				}
			}

			return result, httpHeader, err
		}

		if trace, hit := h.blockTraceCache.Get(traceCacheKey{
			blockHash: *block.Hash,
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
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(throttledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]TracedBlockTransaction, 0, len(executionResult.Traces))
	for index, trace := range executionResult.Traces {
		var L1Gas, L1DataGas, L2Gas uint64
		if gc := executionResult.GasConsumed; gc != nil {
			L1Gas = gc[index].L1Gas
			L1DataGas = gc[index].L1DataGas
			L2Gas = gc[index].L2Gas
		}
		executionResult.Traces[index].ExecutionResources = &vm.ExecutionResources{
			L1Gas:                L1Gas,
			L1DataGas:            L1DataGas,
			L2Gas:                L2Gas,
			ComputationResources: trace.TotalComputationResources(),
			DataAvailability: &vm.DataAvailability{
				L1Gas:     executionResult.DataAvailability[index].L1Gas,
				L1DataGas: executionResult.DataAvailability[index].L1DataGas,
			},
		}
		result = append(result, TracedBlockTransaction{
			TraceRoot:       &executionResult.Traces[index],
			TransactionHash: block.Transactions[index].Hash(),
		})
	}

	if !isPending {
		h.blockTraceCache.Add(traceCacheKey{
			blockHash: *block.Hash,
		}, result)
	}

	return result, httpHeader, nil
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
			return nil, rpccore.ErrInternal.CloneWithData(throttledVMErr)
		}
		return nil, makeContractError(err)
	}
	return res, nil
}
