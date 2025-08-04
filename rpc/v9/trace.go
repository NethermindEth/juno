package rpcv9

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commonstate"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var traceFallbackVersion = semver.MustParse("0.13.1")

const excludedVersion = "0.13.1.1"

type TransactionTrace struct {
	Type                  TransactionType     `json:"type"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"`
	FunctionInvocation    *ExecuteInvocation  `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`
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
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (TransactionTrace, http.Header, *jsonrpc.Error) {
	_, blockHash, _, err := h.bcReader.Receipt(&hash)
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return TransactionTrace{}, httpHeader, rpccore.ErrTxnHashNotFound
	}

	isPreConfirmed := blockHash == nil
	if !isPreConfirmed {
		block, err := h.bcReader.BlockByHash(blockHash)
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return TransactionTrace{}, httpHeader, rpccore.ErrTxnHashNotFound
		}

		txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
			return tx.Hash().Equal(&hash)
		})

		if txIndex == -1 {
			return TransactionTrace{}, httpHeader, rpccore.ErrTxnHashNotFound
		}

		blockTraces, httphttpHeader, rpcErr := h.traceBlockTransactions(ctx, block)
		if rpcErr != nil {
			return TransactionTrace{}, httphttpHeader, rpcErr
		}
		return *blockTraces[txIndex].TraceRoot, httphttpHeader, nil
	}

	var pendingData core.PendingData
	pendingData, err = h.PendingData()
	if err != nil {
		// for traceTransaction handlers there is no block not found error
		return TransactionTrace{}, httpHeader, rpccore.ErrTxnHashNotFound
	}
	block := pendingData.GetBlock()

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(&hash)
	})
	if txIndex == -1 {
		return TransactionTrace{}, httpHeader, rpccore.ErrTxnHashNotFound
	}

	switch v := pendingData.Variant(); v {
	case core.PendingBlockVariant:
		blockTraces, httphttpHeader, rpcErr := h.traceBlockTransactions(ctx, block)
		if rpcErr != nil {
			return TransactionTrace{}, httphttpHeader, rpcErr
		}
		return *blockTraces[txIndex].TraceRoot, httphttpHeader, nil
	case core.PreConfirmedBlockVariant:
		return h.tracePreConfirmedTransaction(block, txIndex)
	default:
		panic(fmt.Errorf("unknown pending data variant: %v", v))
	}
}

func (h *Handler) tracePreConfirmedTransaction(block *core.Block, txIndex int) (TransactionTrace, http.Header, *jsonrpc.Error) {
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")
	state, err := h.syncReader.PendingStateBeforeIndex(txIndex)
	if err != nil {
		return TransactionTrace{}, httpHeader, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	var classes []core.Class
	paidFeesOnL1 := []*felt.Felt{}

	transaction := block.Transactions[txIndex]
	switch tx := transaction.(type) {
	/// TODO(Ege): decide what to do with this, should be an edge case
	case *core.DeclareTransaction:
		class, stateErr := state.Class(tx.ClassHash)
		if stateErr != nil {
			return TransactionTrace{}, httpHeader, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
		}
		classes = append(classes, class.Class)
	case *core.L1HandlerTransaction:
		var fee felt.Felt
		paidFeesOnL1 = append(paidFeesOnL1, fee.SetUint64(1))
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(block.Number)
	if err != nil {
		return TransactionTrace{}, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}
	network := h.bcReader.Network()
	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	executionResult, err := h.vm.Execute([]core.Transaction{transaction}, classes, paidFeesOnL1,
		&blockInfo, state, network, false, false, false, true, false)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return TransactionTrace{}, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return TransactionTrace{}, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	// Adapt vm transaction trace to rpc v9 trace and add root level execution resources
	trace := AdaptVMTransactionTrace(&executionResult.Traces[0])

	trace.ExecutionResources = &ExecutionResources{
		InnerExecutionResources: InnerExecutionResources{
			L1Gas: executionResult.GasConsumed[0].L1Gas,
			L2Gas: executionResult.GasConsumed[0].L2Gas,
		},
		L1DataGas: executionResult.GasConsumed[0].L1DataGas,
	}

	return trace, httpHeader, nil
}

// TraceBlockTransactions returns the trace for a given blockID
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L108
func (h *Handler) TraceBlockTransactions(
	ctx context.Context, id *BlockID,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	if id.IsPreConfirmed() {
		httpHeader := http.Header{}
		httpHeader.Set(ExecutionStepsHeader, "0")
		return nil, httpHeader, rpccore.ErrCallOnPreConfirmed
	}

	block, rpcErr := h.blockByID(id)
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

			// fgw doesn't provide this data in traces endpoint. So, we get it from our block receipts
			txTotalGasConsumed := make(map[felt.Felt]core.GasConsumed, len(block.Receipts))
			for _, receipt := range block.Receipts {
				if receipt.ExecutionResources == nil {
					continue
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

			// For every trace in block, add execution resources on root level
			for index, trace := range result {
				tgs := txTotalGasConsumed[*trace.TransactionHash]

				result[index].TraceRoot.ExecutionResources = &ExecutionResources{
					InnerExecutionResources: InnerExecutionResources{
						L1Gas: tgs.L1Gas,
						L2Gas: tgs.L2Gas,
					},
					L1DataGas: tgs.L1DataGas,
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

	state, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return nil, httpHeader, rpccore.ErrBlockNotFound
	}

	var headState commonstate.StateReader
	if isPending {
		headState, err = h.PendingState()
	} else {
		headState, err = h.bcReader.HeadState()
	}
	if err != nil {
		return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

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
		&blockInfo, state, network, false, false, false, true, false)

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
		h.blockTraceCache.Add(rpccore.TraceCacheKey{
			BlockHash: *block.Hash,
		}, result)
	}

	return result, httpHeader, nil
}

func (h *Handler) fetchTraces(ctx context.Context, blockHash *felt.Felt) ([]TracedBlockTransaction, *jsonrpc.Error) {
	blockID := BlockIDFromHash(blockHash)
	rpcBlock, err := h.BlockWithTxs(&blockID)
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

	traces, aErr := AdaptFeederBlockTrace(rpcBlock, blockTrace)
	if aErr != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(aErr.Error())
	}

	return traces, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L579
func (h *Handler) Call(funcCall *FunctionCall, id *BlockID) ([]*felt.Felt, *jsonrpc.Error) {
	state, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	header, rpcErr := h.blockHeaderByID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	classHash, err := state.ContractClassHash(&funcCall.ContractAddress)
	if err != nil {
		return nil, rpccore.ErrContractNotFound
	}

	declaredClass, err := state.Class(&classHash)
	if err != nil {
		return nil, rpccore.ErrClassHashNotFound
	}

	sierraVersion := declaredClass.Class.SierraVersion()

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	res, err := h.vm.Call(&vm.CallInfo{
		ContractAddress: &funcCall.ContractAddress,
		Selector:        &funcCall.EntryPointSelector,
		Calldata:        funcCall.Calldata,
		ClassHash:       &classHash,
	}, &vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}, state, h.bcReader.Network(), h.callMaxSteps, sierraVersion, true, false)
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
