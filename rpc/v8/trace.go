package rpcv8

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type TransactionTrace struct {
	Type                  TransactionType     `json:"type"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`
	StateDiff             *rpcv6.StateDiff    `json:"state_diff,omitempty"`
	ExecutionResources    *ExecutionResources `json:"execution_resources"`
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

type FunctionInvocation struct {
	ContractAddress    felt.Felt                    `json:"contract_address"`
	EntryPointSelector *felt.Felt                   `json:"entry_point_selector"`
	Calldata           []felt.Felt                  `json:"calldata"`
	CallerAddress      felt.Felt                    `json:"caller_address"`
	ClassHash          *felt.Felt                   `json:"class_hash"`
	EntryPointType     string                       `json:"entry_point_type"` // shouldnt we put it as enum here ?
	CallType           string                       `json:"call_type"`        // shouldnt we put it as enum here ?
	Result             []felt.Felt                  `json:"result"`
	Calls              []FunctionInvocation         `json:"calls"`
	Events             []rpcv6.OrderedEvent         `json:"events"`
	Messages           []rpcv6.OrderedL2toL1Message `json:"messages"`
	ExecutionResources *InnerExecutionResources     `json:"execution_resources"`
	IsReverted         bool                         `json:"is_reverted"`
}

/****************************************************
		Tracing Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/1ae810e0137cc5d175ace4554892a4f43052be56/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (*TransactionTrace, http.Header, *jsonrpc.Error) {
	_, blockHash, _, err := h.bcReader.Receipt(&hash)
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, httpHeader, rpccore.ErrTxnHashNotFound
	}

	var block *core.Block
	isPendingBlock := blockHash == nil
	if isPendingBlock {
		var pending core.PendingData
		pending, err = h.PendingData()
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, httpHeader, rpccore.ErrTxnHashNotFound
		}
		block = pending.GetBlock()
	} else {
		block, err = h.bcReader.BlockByHash(blockHash)
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, httpHeader, rpccore.ErrTxnHashNotFound
		}
	}

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(&hash)
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

func (h *Handler) TraceBlockTransactions(
	ctx context.Context, id *BlockID,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, defaultExecutionHeader(), rpcErr
	}

	return h.traceBlockTransactions(ctx, block)
}

// traceBlockTransactions gets the trace for a block. The block will always be traced locally except
// on specific case such as with Starknet version 0.13.2 or lower or when it is certain range
func (h *Handler) traceBlockTransactions(
	ctx context.Context, block *core.Block,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	isPending := block.Hash == nil
	if !isPending {
		// Check if it was already traced
		traces, hit := h.blockTraceCache.Get(rpccore.TraceCacheKey{BlockHash: *block.Hash})
		if hit {
			return traces, defaultExecutionHeader(), nil
		}

		// Check if the trace should be provided by the feeder gateway
		blockVer, err := core.ParseBlockVersion(block.ProtocolVersion)
		if err != nil {
			return nil,
				defaultExecutionHeader(),
				rpccore.ErrUnexpectedError.CloneWithData(err.Error())
		}
		// We rely on the feeder gateway for Starknet version strictly older than "0.13.1.1"
		fetchFromFeederGW := blockVer.LessThan(core.Ver0_13_2) &&
			block.ProtocolVersion != "0.13.1.1"
		// This specific block range caused a re-org, also related with Cairo 0 and we have to
		// depend on the Sequencer to provide the correct traces
		fetchFromFeederGW = fetchFromFeederGW ||
			(block.Number >= 1943705 &&
				block.Number <= 1952704 &&
				*h.bcReader.Network() == utils.Mainnet)

		if fetchFromFeederGW {
			traces, err := h.fetchTracesFromFeederGateway(ctx, block)
			if err != nil {
				return nil, defaultExecutionHeader(), err
			}
			h.blockTraceCache.Add(rpccore.TraceCacheKey{BlockHash: *block.Hash}, traces)
			return traces, defaultExecutionHeader(), nil
		}
	}

	return h.traceBlockTransactionWithVM(block)
}

// `traceBlockTransactionWithVM` traces a block and stores it in the block cache
func (h *Handler) traceBlockTransactionWithVM(block *core.Block) (
	[]TracedBlockTransaction, http.Header, *jsonrpc.Error,
) {
	httpHeader := defaultExecutionHeader()
	state, closer, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return nil, httpHeader, rpccore.ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in traceBlockTransactions")

	var (
		headState       core.CommonStateReader
		headStateCloser blockchain.StateCloser
	)

	isPending := block.Hash == nil
	if isPending {
		headState, headStateCloser, err = h.PendingState()
	} else {
		headState, headStateCloser, err = h.bcReader.HeadState()
	}
	if err != nil {
		return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	var classes []core.ClassDefinition
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

	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	executionResult, err := h.vm.Execute(block.Transactions, classes, paidFeesOnL1,
		&blockInfo, state, false, false, false, true, false, false)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]TracedBlockTransaction, len(executionResult.Traces))
	// Adapt every vm transaction trace to rpc v8 trace and add root level execution resources
	for index := range executionResult.Traces {
		trace := utils.HeapPtr(AdaptVMTransactionTrace(&executionResult.Traces[index]))

		trace.ExecutionResources = &ExecutionResources{
			InnerExecutionResources: InnerExecutionResources{
				L1Gas: executionResult.GasConsumed[index].L1Gas,
				L2Gas: executionResult.GasConsumed[index].L2Gas,
			},
			L1DataGas: executionResult.GasConsumed[index].L1DataGas,
		}

		result[index] = TracedBlockTransaction{
			TraceRoot:       trace,
			TransactionHash: block.Transactions[index].Hash(),
		}
	}

	if !isPending {
		h.blockTraceCache.Add(rpccore.TraceCacheKey{BlockHash: *block.Hash}, result)
	}

	return result, httpHeader, nil
}

func (h *Handler) fetchTracesFromFeederGateway(
	ctx context.Context, block *core.Block,
) ([]TracedBlockTransaction, *jsonrpc.Error) {
	// todo(rdr): this feels unnatural, why if I have the `core.Block` should I still
	// try to go for the rpcBlock? Ideally we extract all the info directly from `core.Block`
	blockID := BlockIDFromHash(block.Hash)
	rpcBlock, rpcErr := h.BlockWithTxs(&blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	if h.feederClient == nil {
		return nil, rpccore.ErrInternal.CloneWithData("no feeder client configured")
	}

	blockTrace, err := h.feederClient.BlockTrace(ctx, block.Hash.String())
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	traces, err := AdaptFeederBlockTrace(rpcBlock, blockTrace)
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	traces = fillFeederGatewayData(traces, block.Receipts)

	return traces, nil
}

// `fillFeederGatewayData` mutates the `traces` argument and fill it with the data from `receipts` which
// the Feeder Gateway doesn't provide by default
func fillFeederGatewayData(
	traces []TracedBlockTransaction, receipts []*core.TransactionReceipt,
) []TracedBlockTransaction {
	totalGasConsumed := make(map[felt.Felt]core.GasConsumed, len(receipts))
	for _, re := range receipts {
		if re.ExecutionResources == nil {
			continue
		}

		if reGasConsumed := re.ExecutionResources.TotalGasConsumed; reGasConsumed != nil {
			tgs := core.GasConsumed{
				L1Gas:     reGasConsumed.L1Gas,
				L1DataGas: reGasConsumed.L1DataGas,
				L2Gas:     reGasConsumed.L2Gas,
			}
			totalGasConsumed[*re.TransactionHash] = tgs
		}
	}

	// For every trace in block, add execution resources on root level
	for index, trace := range traces {
		tgs := totalGasConsumed[*trace.TransactionHash]

		traces[index].TraceRoot.ExecutionResources = &ExecutionResources{
			InnerExecutionResources: InnerExecutionResources{
				L1Gas: tgs.L1Gas,
				L2Gas: tgs.L2Gas,
			},
			L1DataGas: tgs.L1DataGas,
		}
	}

	return traces
}

func defaultExecutionHeader() http.Header {
	header := http.Header{}
	header.Set(ExecutionStepsHeader, "0")
	return header
}

// https://github.com/starkware-libs/starknet-specs/blob/e0b76ed0d8d8eba405e182371f9edac8b2bcbc5a/api/starknet_api_openrpc.json#L401-L445
func (h *Handler) Call(funcCall *FunctionCall, id *BlockID) ([]*felt.Felt, *jsonrpc.Error) {
	state, closer, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_call")

	header, rpcErr := h.blockHeaderByID(id)
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

	res, err := h.vm.Call(
		&vm.CallInfo{
			ContractAddress: &funcCall.ContractAddress,
			Selector:        &funcCall.EntryPointSelector,
			Calldata:        funcCall.Calldata.Data,
			ClassHash:       &classHash,
		},
		&vm.BlockInfo{
			Header:                header,
			BlockHashToBeRevealed: blockHashToBeRevealed,
		},
		state,
		h.callMaxSteps,
		h.callMaxGas,
		true,
		false,
	)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, MakeContractError(json.RawMessage(err.Error()))
	}
	if res.ExecutionFailed {
		// the blockifier 0.13.4 update requires us to check if the execution failed,
		// and if so, return ErrEntrypointNotFound if res.Result[0]==EntrypointNotFoundFelt,
		// otherwise we should wrap the result in ErrContractError
		var strErr string
		if len(res.Result) != 0 {
			if res.Result[0].String() == rpccore.EntrypointNotFoundFelt {
				return nil, rpccore.ErrEntrypointNotFound
			}
			strErr = `"` + utils.FeltArrToString(res.Result) + `"`
		}
		// Todo: There is currently no standardised way to format these error messages
		return nil, MakeContractError(json.RawMessage(strErr))
	}
	return res.Result, nil
}
