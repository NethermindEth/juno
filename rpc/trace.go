package rpc

import (
	"context"
	"errors"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var traceFallbackVersion = semver.MustParse("0.12.3")

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
	return h.traceTransaction(ctx, &hash, false)
}

func (h *Handler) TraceTransactionV0_6(ctx context.Context, hash felt.Felt) (*vm.TransactionTrace, *jsonrpc.Error) {
	return h.traceTransaction(ctx, &hash, true)
}

func (h *Handler) traceTransaction(ctx context.Context, hash *felt.Felt, v0_6Response bool) (*vm.TransactionTrace, *jsonrpc.Error) {
	_, _, blockNumber, err := h.bcReader.Receipt(hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	block, err := h.bcReader.BlockByNumber(blockNumber)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(hash)
	})
	if txIndex == -1 {
		return nil, ErrTxnHashNotFound
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

	return h.traceBlockTransactions(ctx, block, false)
}

func (h *Handler) TraceBlockTransactionsV0_6(ctx context.Context, id BlockID) ([]TracedBlockTransaction, *jsonrpc.Error) {
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
			return nil, ErrUnexpectedError.CloneWithData(err.Error())
		} else if blockVer.Compare(traceFallbackVersion) != 1 {
			// version <= 0.12.3
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
		return nil, ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in traceBlockTransactions")

	var (
		headState       core.StateReader
		headStateCloser blockchain.StateCloser
	)
	if isPending {
		headState, headStateCloser, err = h.bcReader.PendingState()
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
		return nil, ErrInternal.CloneWithData(err)
	}
	network := h.bcReader.Network()
	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	useBlobData := !v0_6Response
	overallFees, dataGasConsumed, traces, err := h.vm.Execute(block.Transactions, classes, paidFeesOnL1, &blockInfo, state, network, false,
		false, false, useBlobData)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, ErrInternal.CloneWithData(throttledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, ErrUnexpectedError.CloneWithData(err.Error())
	}

	var result []TracedBlockTransaction
	for index, trace := range traces {
		if !v0_6Response {
			feeUnit := feeUnit(block.Transactions[index])

			gasPrice := header.GasPrice
			if feeUnit == FRI {
				if gasPrice = header.GasPriceSTRK; gasPrice == nil {
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

			dataGasFee := new(felt.Felt).Mul(dataGasConsumed[index], dataGasPrice)
			gasConsumed := new(felt.Felt).Sub(overallFees[index], dataGasFee)
			gasConsumed = gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

			executionResources := trace.TotalExecutionResources()
			executionResources.DataAvailability = vm.NewDataAvailability(gasConsumed, dataGasConsumed[index],
				header.L1DAMode)
			traces[index].ExecutionResources = executionResources
		}
		result = append(result, TracedBlockTransaction{
			TraceRoot:       &traces[index],
			TransactionHash: block.Transactions[index].Hash(),
		})
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
		return nil, ErrInternal.CloneWithData("no feeder client configured")
	}

	blockTrace, fErr := h.feederClient.BlockTrace(ctx, blockHash.String())
	if fErr != nil {
		return nil, ErrUnexpectedError.CloneWithData(fErr.Error())
	}

	traces, aErr := adaptBlockTrace(rpcBlock, blockTrace)
	if aErr != nil {
		return nil, ErrUnexpectedError.CloneWithData(aErr.Error())
	}

	return traces, nil
}
