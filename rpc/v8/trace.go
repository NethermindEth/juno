package rpcv8

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/rpc/tracecache"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/throttler"
	"github.com/NethermindEth/juno/vm"
)

type TransactionTrace struct {
	Type                  TransactionType     `json:"type"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`
	StateDiff             *StateDiff          `json:"state_diff,omitempty"`
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
	ContractAddress    felt.Felt                `json:"contract_address"`
	EntryPointSelector *felt.Felt               `json:"entry_point_selector"`
	Calldata           []felt.Felt              `json:"calldata"`
	CallerAddress      felt.Felt                `json:"caller_address"`
	ClassHash          *felt.Felt               `json:"class_hash"`
	EntryPointType     string                   `json:"entry_point_type"`
	CallType           string                   `json:"call_type"`
	Result             []felt.Felt              `json:"result"`
	Calls              []FunctionInvocation     `json:"calls"`
	Events             []OrderedEvent           `json:"events"`
	Messages           []OrderedL2toL1Message   `json:"messages"`
	ExecutionResources *InnerExecutionResources `json:"execution_resources"`
	IsReverted         bool                     `json:"is_reverted"`
}

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L3135
type FunctionCall struct {
	ContractAddress    felt.Felt      `json:"contract_address"`
	EntryPointSelector felt.Felt      `json:"entry_point_selector"`
	Calldata           CalldataInputs `json:"calldata"`
}

type OrderedEvent struct {
	Order uint64      `json:"order"`
	Keys  []felt.Felt `json:"keys"`
	Data  []felt.Felt `json:"data"`
}

type OrderedL2toL1Message struct {
	Order   uint64      `json:"order"`
	From    *felt.Felt  `json:"from_address"`
	To      *felt.Felt  `json:"to_address"`
	Payload []felt.Felt `json:"payload"`
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
		pending, err := h.Pending()
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

	var target *tracecache.TransactionTarget
	if !isPendingBlock {
		target = &tracecache.TransactionTarget{Index: uint64(txIndex), Hash: &hash}
	}
	traceResults, header, traceBlockErr := h.traceBlockTransactions(ctx, block, target)
	if traceBlockErr != nil {
		return nil, header, traceBlockErr
	}

	var trace TransactionTrace
	adaptCachedTrace(&traceResults.Traces[txIndex], &trace)
	return &trace, header, nil
}

func (h *Handler) TraceBlockTransactions(
	ctx context.Context, id *BlockID,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, defaultExecutionHeader(), rpcErr
	}

	traces, httpHeader, rpcErr := h.traceBlockTransactions(ctx, block, nil)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}
	return adaptCachedTraces(traces), httpHeader, nil
}

// traceBlockTransactions caches local or feeder traces; pending blocks bypass the cache.
//
//nolint:gocyclo // Preserve v8 pending/finalized orchestration and error ordering in one place.
func (h *Handler) traceBlockTransactions(
	ctx context.Context, block *core.Block, target *tracecache.TransactionTarget,
) (*tracecache.BlockTrace, http.Header, *jsonrpc.Error) {
	isPending := block.Hash == nil
	var lease *tracecache.Lease[felt.Felt, *tracecache.BlockTrace]
	var cached *tracecache.BlockTrace
	var plan *tracecache.Range
	if !isPending {
		previous, acquiredLease, err := h.blockTraceCache.AcquireWithCondition(
			ctx,
			block.Hash,
			func(b *tracecache.BlockTrace) bool {
				return b.CoversTarget(target, false)
			},
		)
		cached = previous
		if err != nil {
			return nil, defaultExecutionHeader(), rpccore.ErrUnexpectedError.CloneWithData(err.Error())
		}
		if acquiredLease == nil {
			if cached.ValidateTarget(target) != nil {
				return nil, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
			}
			return cached, defaultExecutionHeader(), nil
		}
		lease = acquiredLease
		defer lease.Release()

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
				*h.bcReader.Network() == networks.Mainnet)

		if fetchFromFeederGW {
			traces, err := h.fetchTracesFromFeederGateway(ctx, block)
			if err != nil {
				return nil, defaultExecutionHeader(), err
			}
			if traces.ValidateTarget(target) != nil {
				return nil, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
			}
			lease.Publish(traces)
			return traces, defaultExecutionHeader(), nil
		}
	}

	if !isPending {
		var planErr error
		plan, planErr = tracecache.PlanRange(cached, block.Transactions, target, false)
		if planErr != nil {
			if errors.Is(planErr, tracecache.ErrTargetNotFound) {
				return nil, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
			}
			return nil, defaultExecutionHeader(), rpccore.ErrUnexpectedError.CloneWithData(planErr.Error())
		}
	}
	traces, httpHeader, rpcErr := h.traceBlockTransactionWithVM(block, plan)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}
	if lease != nil {
		combined, combineErr := plan.Combine(traces)
		if combineErr != nil {
			return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(combineErr.Error())
		}
		traces = combined
		lease.Publish(traces)
	}
	return traces, httpHeader, nil
}

func (h *Handler) traceBlockTransactionWithVM(block *core.Block, plan *tracecache.Range) (
	*tracecache.BlockTrace, http.Header, *jsonrpc.Error,
) {
	httpHeader := defaultExecutionHeader()
	transactions := block.Transactions
	if plan != nil {
		transactions = transactions[plan.Start:plan.End]
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

	headState, headStateCloser, err = h.bcReader.HeadState()
	if err != nil {
		return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	if plan != nil {
		state, err = plan.ResumeState(state, headState, block.Number)
		if err != nil {
			return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
	}

	var classes []core.ClassDefinition
	paidFeesOnL1 := []*felt.Felt{}

	for _, transaction := range transactions {
		switch tx := transaction.(type) {
		case *core.DeclareTransaction:
			class, stateErr := headState.Class(tx.ClassHash)
			if stateErr != nil {
				return nil, httpHeader, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
			}
			classes = append(classes, class.Class)
		case *core.L1HandlerTransaction:
			// TODO (granza): use real L1 message fee.
			paidFeesOnL1 = append(paidFeesOnL1, &felt.One)
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

	executionResult, err := h.vm.Trace(transactions, classes, paidFeesOnL1,
		&blockInfo, state, vm.TraceOptions{})

	if plan != nil {
		err = tracecache.OffsetExecutionError(err, plan.Start)
	}
	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if err != nil {
		if errors.Is(err, throttler.ErrResourceBusy) {
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	result, packErr := tracecache.FromVM(transactions, &executionResult, false)
	if packErr != nil {
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(packErr.Error())
	}
	return result, httpHeader, nil
}

func (h *Handler) fetchTracesFromFeederGateway(
	ctx context.Context, block *core.Block,
) (*tracecache.BlockTrace, *jsonrpc.Error) {
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

	kinds := make([]vm.TransactionType, len(rpcBlock.Transactions))
	for i := range rpcBlock.Transactions {
		kinds[i] = vm.TransactionType(rpcBlock.Transactions[i].Type)
	}
	traces, err := tracecache.FromFeeder(kinds, block.Receipts, &blockTrace)
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	return traces, nil
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
		if errors.Is(err, throttler.ErrResourceBusy) {
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
