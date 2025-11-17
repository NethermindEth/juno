package rpcv7

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
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
	Type                  TransactionType           `json:"type"`
	ValidateInvocation    *rpcv6.FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *rpcv6.ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"`
	FeeTransferInvocation *rpcv6.FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *rpcv6.FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"`
	FunctionInvocation    *rpcv6.FunctionInvocation `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`
	StateDiff             *rpcv6.StateDiff          `json:"state_diff,omitempty"`
	ExecutionResources    *ExecutionResources       `json:"execution_resources"`
}

func (t *TransactionTrace) TotalComputationResources() ComputationResources {
	total := ComputationResources{}

	for _, invocation := range t.allInvocations() {
		r := invocation.ExecutionResources

		total.Pedersen += r.Pedersen
		total.RangeCheck += r.RangeCheck
		total.Bitwise += r.Bitwise
		total.Ecdsa += r.Ecdsa
		total.EcOp += r.EcOp
		total.Keccak += r.Keccak
		total.Poseidon += r.Poseidon
		total.SegmentArena += r.SegmentArena
		total.MemoryHoles += r.MemoryHoles
		total.Steps += r.Steps
	}

	return total
}

func (t *TransactionTrace) allInvocations() []*rpcv6.FunctionInvocation {
	var executeInvocation *rpcv6.FunctionInvocation
	if t.ExecuteInvocation != nil {
		executeInvocation = t.ExecuteInvocation.FunctionInvocation
	}

	return slices.DeleteFunc([]*rpcv6.FunctionInvocation{
		t.ConstructorInvocation,
		t.ValidateInvocation,
		t.FeeTransferInvocation,
		executeInvocation,
		t.FunctionInvocation,
	}, func(i *rpcv6.FunctionInvocation) bool { return i == nil })
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

			txDataAvailability := make(map[felt.Felt]DataAvailability, len(block.Receipts))
			// Get Data Availability from every receipt in block to store them in traces
			for _, receipt := range block.Receipts {
				if receipt.ExecutionResources == nil {
					continue
				}

				if receiptDA := receipt.ExecutionResources.DataAvailability; receiptDA != nil {
					da := DataAvailability{
						L1Gas:     receiptDA.L1Gas,
						L1DataGas: receiptDA.L1DataGas,
					}
					txDataAvailability[*receipt.TransactionHash] = da
				}
			}

			// Add execution resources on root level for every trace in block
			for index := range result {
				trace := &result[index]

				// fgw doesn't provide this data in traces endpoint
				// some receipts don't have data availability data in this case we don't
				da := txDataAvailability[*trace.TransactionHash]

				trace.TraceRoot.ExecutionResources = &ExecutionResources{
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
		headState       commonstate.StateReader
		headStateCloser blockchain.StateCloser
	)
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

	executionResult, err := h.vm.Execute(
		block.Transactions,
		classes,
		paidFeesOnL1,
		&blockInfo,
		state,
		false, false, false, false, false, false)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(throttledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]TracedBlockTransaction, len(executionResult.Traces))
	// Add execution resources on root level for every trace in block
	for index := range executionResult.Traces {
		trace := utils.HeapPtr(AdaptVMTransactionTrace(&executionResult.Traces[index]))

		// Add root level execution resources
		trace.ExecutionResources = &ExecutionResources{
			ComputationResources: trace.TotalComputationResources(),
			DataAvailability: &DataAvailability{
				L1Gas:     executionResult.DataAvailability[index].L1Gas,
				L1DataGas: executionResult.DataAvailability[index].L1DataGas,
			},
		}

		result[index] = TracedBlockTransaction{
			TraceRoot:       trace,
			TransactionHash: block.Transactions[index].Hash(),
		}
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

	traces, aErr := AdaptFeederBlockTrace(rpcBlock, blockTrace)
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
		false,
		false,
	)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(throttledVMErr)
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
				strErr = fmt.Sprintf(
					rpccore.ErrEPSNotFound,
					funcCall.EntryPointSelector.String(),
				)
			} else {
				strErr = utils.FeltArrToString(res.Result)
			}
			strErr = `"` + strErr + `"`
		}
		// Todo: There is currently no standardised way to format these error messages
		return nil, MakeContractError(json.RawMessage(strErr))
	}
	return res.Result, nil
}
