package rpcv6

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var traceFallbackVersion = semver.MustParse("0.13.1")

const excludedVersion = "0.13.1.1"

const ExecutionStepsHeader string = "X-Cairo-Steps"

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
	return &vm.ExecutionResources{
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
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (*vm.TransactionTrace, *jsonrpc.Error) {
	return h.traceTransaction(ctx, &hash, true)
}

func (h *Handler) traceTransaction(ctx context.Context, hash *felt.Felt, v0_6Response bool) (*vm.TransactionTrace, *jsonrpc.Error) {
	_, blockHash, _, err := h.bcReader.Receipt(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	var block *core.Block
	isPendingBlock := blockHash == nil
	if isPendingBlock {
		var pending *sync.Pending
		pending, err = h.syncReader.Pending()
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, rpccore.ErrTxnHashNotFound
		}
		block = pending.Block
	} else {
		block, err = h.bcReader.BlockByHash(blockHash)
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, rpccore.ErrTxnHashNotFound
		}
	}

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(hash)
	})
	if txIndex == -1 {
		return nil, rpccore.ErrTxnHashNotFound
	}

	traceResults, traceBlockErr := h.traceBlockTransactions(ctx, block, v0_6Response)
	if traceBlockErr != nil {
		return nil, traceBlockErr
	}

	return traceResults[txIndex].TraceRoot, nil
}

func (h *Handler) TraceBlockTransactions(ctx context.Context, id BlockID) ([]TracedBlockTransaction, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return h.traceBlockTransactions(ctx, block, true)
}

func (h *Handler) traceBlockTransactions(ctx context.Context, block *core.Block, v0_6Response bool, //nolint: gocyclo, funlen
) ([]TracedBlockTransaction, *jsonrpc.Error) {
	isPending := block.Hash == nil
	if !isPending {
		if blockVer, err := core.ParseBlockVersion(block.ProtocolVersion); err != nil {
			return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
		} else if blockVer.Compare(traceFallbackVersion) != 1 && block.ProtocolVersion != excludedVersion {
			// version <= 0.13.1 and not 0.13.1.1 or forcing fetch some blocks from feeder gateway
			return h.fetchTraces(ctx, block.Hash)
		}

		if trace, hit := h.blockTraceCache.Get(traceCacheKey{
			blockHash:    *block.Hash,
			v0_6Response: v0_6Response,
		}); hit {
			return trace, nil
		}
	}

	state, closer, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return nil, rpccore.ErrBlockNotFound
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
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	var classes []core.Class
	paidFeesOnL1 := []*felt.Felt{}

	for _, transaction := range block.Transactions {
		switch tx := transaction.(type) {
		case *core.DeclareTransaction:
			class, stateErr := headState.Class(tx.ClassHash)
			if stateErr != nil {
				return nil, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
			}
			classes = append(classes, class.Class)
		case *core.L1HandlerTransaction:
			var fee felt.Felt
			paidFeesOnL1 = append(paidFeesOnL1, fee.SetUint64(1))
		}
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(block.Number)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}
	network := h.bcReader.Network()
	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	overallFees, dataGasConsumed, traces, _, err := h.vm.Execute(block.Transactions, classes, paidFeesOnL1, &blockInfo, state, network, false,
		false, false)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]TracedBlockTransaction, len(traces))
	for index, trace := range traces {
		if !v0_6Response {
			feeUnit := feeUnit(block.Transactions[index])

			gasPrice := header.L1GasPriceETH
			if feeUnit == FRI {
				if gasPrice = header.L1GasPriceSTRK; gasPrice == nil {
					gasPrice = &felt.Zero
				}
			}

			dataGasPrice := &felt.Zero
			if header.L1DataGasPrice != nil {
				switch feeUnit {
				case FRI:
					dataGasPrice = header.L1DataGasPrice.PriceInFri
				case WEI:
					dataGasPrice = header.L1DataGasPrice.PriceInWei
				}
			}

			l1DAGas := new(felt.Felt).SetUint64(dataGasConsumed[index].L1DataGas)
			dataGasFee := new(felt.Felt).Mul(l1DAGas, dataGasPrice)
			gasConsumed := new(felt.Felt).Sub(overallFees[index], dataGasFee)
			gasConsumed = gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

			executionResources := trace.TotalExecutionResources()
			da := vm.NewDataAvailability(gasConsumed, l1DAGas,
				header.L1DAMode)
			executionResources.DataAvailability = &da
			traces[index].ExecutionResources = executionResources
		}
		result[index] = TracedBlockTransaction{
			TraceRoot:       &traces[index],
			TransactionHash: block.Transactions[index].Hash(),
		}
	}

	if !isPending {
		h.blockTraceCache.Add(traceCacheKey{
			blockHash:    *block.Hash,
			v0_6Response: v0_6Response,
		}, result)
	}

	return result, nil
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
	return h.call(funcCall, id)
}

func (h *Handler) call(funcCall FunctionCall, id BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
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
	}, state, h.bcReader.Network(), h.callMaxSteps)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, makeContractError(err)
	}
	return res, nil
}
