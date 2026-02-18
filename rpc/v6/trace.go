package rpcv6

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var traceFallbackVersion = semver.MustParse("0.13.1")

const excludedVersion = "0.13.1.1"

const ExecutionStepsHeader string = "X-Cairo-Steps"

type TransactionTrace struct {
	Type                  TransactionType     `json:"type"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`
	StateDiff             *StateDiff          `json:"state_diff,omitempty"`
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
	ContractAddress    felt.Felt              `json:"contract_address"`
	EntryPointSelector *felt.Felt             `json:"entry_point_selector"`
	Calldata           []felt.Felt            `json:"calldata"`
	CallerAddress      felt.Felt              `json:"caller_address"`
	ClassHash          *felt.Felt             `json:"class_hash"`
	EntryPointType     string                 `json:"entry_point_type"`
	CallType           string                 `json:"call_type"`
	Result             []felt.Felt            `json:"result"`
	Calls              []FunctionInvocation   `json:"calls"`
	Events             []OrderedEvent         `json:"events"`
	Messages           []OrderedL2toL1Message `json:"messages"`
	ExecutionResources *ComputationResources  `json:"execution_resources"`
}

type OrderedEvent struct {
	Order uint64       `json:"order"`
	Keys  []*felt.Felt `json:"keys"`
	Data  []*felt.Felt `json:"data"`
}

type OrderedL2toL1Message struct {
	Order   uint64       `json:"order"`
	From    *felt.Felt   `json:"from_address"`
	To      *felt.Felt   `json:"to_address"`
	Payload []*felt.Felt `json:"payload"`
}

/****************************************************
		Tracing Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/1ae810e0137cc5d175ace4554892a4f43052be56/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (*TransactionTrace, *jsonrpc.Error) {
	_, blockHash, _, err := h.bcReader.Receipt(&hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}
	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	var block *core.Block
	isPendingBlock := blockHash == nil
	if isPendingBlock {
		var pending core.PendingData
		pending, err = h.PendingData()
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, rpccore.ErrTxnHashNotFound
		}
		block = pending.GetBlock()
	} else {
		block, err = h.bcReader.BlockByHash(blockHash)
		if err != nil {
			// for traceTransaction handlers there is no block not found error
			return nil, rpccore.ErrTxnHashNotFound
		}
	}

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(&hash)
	})
	if txIndex == -1 {
		return nil, rpccore.ErrTxnHashNotFound
	}

	traceResults, traceBlockErr := h.traceBlockTransactions(ctx, block)
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

	return h.traceBlockTransactions(ctx, block)
}

func (h *Handler) traceBlockTransactions(ctx context.Context, block *core.Block, //nolint: gocyclo
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
			blockHash: *block.Hash,
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
		headState, headStateCloser, err = h.PendingState()
	} else {
		headState, headStateCloser, err = h.bcReader.HeadState()
	}
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	var classes []core.ClassDefinition
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

	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	executionResults, err := h.vm.Execute(
		block.Transactions,
		classes,
		paidFeesOnL1,
		&blockInfo,
		state,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
	)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result := make([]TracedBlockTransaction, len(executionResults.Traces))
	for i := range executionResults.Traces {
		result[i] = TracedBlockTransaction{
			TraceRoot:       utils.HeapPtr(AdaptVMTransactionTrace(&executionResults.Traces[i])),
			TransactionHash: block.Transactions[i].Hash(),
		}
	}

	if !isPending {
		h.blockTraceCache.Add(traceCacheKey{
			blockHash: *block.Hash,
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

	traces, aErr := AdaptFeederBlockTrace(rpcBlock, blockTrace)
	if aErr != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(aErr.Error())
	}

	return traces, nil
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
		false,
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
