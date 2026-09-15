package rpcv10

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
	"github.com/NethermindEth/juno/core/pending"
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
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"` //nolint:lll // validate tag
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"` //nolint:lll // validate tag
	FunctionInvocation    *ExecuteInvocation  `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`        //nolint:lll // validate tag
	StateDiff             *StateDiff          `json:"state_diff,omitempty"`
	ExecutionResources    *ExecutionResources `json:"execution_resources"`
}

/****************************************************
		Public API Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(
	ctx context.Context, hash *felt.TransactionHash,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	httpHeader := defaultExecutionHeader()

	trace, header, err := h.findAndTraceFinalisedTransaction(ctx, hash)
	if err == nil {
		return trace, header, nil
	}
	if err != rpccore.ErrTxnHashNotFound {
		return TransactionTrace{}, httpHeader, err
	}

	// Not in a finalised block, so try the pre_confirmed chain.
	trace, header, err = h.findAndTraceInPreConfirmed(hash)
	if err != nil {
		return TransactionTrace{}, httpHeader, err
	}
	return trace, header, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/39553a2e5216b7b5e06f6d44368317c0ccd79dfa/api/starknet_api_openrpc.json#L569
func (h *Handler) Call(
	funcCall *FunctionCall,
	id *BlockID,
) ([]*felt.Felt, *jsonrpc.Error) {
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

	blockInfo, rpcErr := h.buildBlockInfo(header)
	if rpcErr != nil {
		return nil, rpcErr
	}

	res, err := h.vm.Call(
		&vm.CallInfo{
			ContractAddress: &funcCall.ContractAddress,
			Selector:        &funcCall.EntryPointSelector,
			Calldata:        funcCall.Calldata.Data,
			ClassHash:       &classHash,
		},
		&blockInfo,
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
				return nil, rpccore.ErrEntrypointNotFoundV0_10
			}
			strErr = `"` + utils.FeltArrToString(res.Result) + `"`
		}
		// Todo: There is currently no standardised way to format these error messages
		return nil, MakeContractError(json.RawMessage(strErr))
	}
	return res.Result, nil
}

// TraceBlockTransactions returns the trace for a given blockID
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L108
func (h *Handler) TraceBlockTransactions(
	ctx context.Context, id *BlockID, traceFlags []TraceFlag,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	if id.IsPreConfirmed() {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpccore.ErrCallOnPreConfirmed
	}

	// Resolve the block id once: the header pins the block number, so the reads that follow it
	// go by number and a tag like `latest` cannot move between them.
	header, rpcErr := h.blockHeaderByID(id)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
	}

	returnInitialReads := slices.Contains(traceFlags, TraceReturnInitialReadsFlag)
	traces, httpHeader, rpcErr := h.traceFinalisedBlock(ctx, header, returnInitialReads)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, httpHeader, rpcErr
	}
	return adaptCachedBlock(traces, returnInitialReads), httpHeader, nil
}

/****************************************************
		Core Tracing Logic
*****************************************************/

// traceTransactionsWithState traces a set of transactions using the provided VM and state readers.
//
// Parameters:
//
//   - runner: The virtual machine used for execution
//
//   - transactions: The transactions to trace
//
//   - executionState: The state used for transaction execution
//
//   - classLookupState: The state used for class definition lookups.
//     This should be at least the state that includes the target block or transaction.
//
//   - blockInfo: Block context for execution
//
//   - returnInitialReads: Whether to return initial reads in the response
func traceTransactionsWithState(
	runner vm.VM,
	transactions []core.Transaction,
	executionState core.StateReader,
	classLookupState core.StateReader,
	blockInfo *vm.BlockInfo,
	returnInitialReads bool,
) (*tracecache.BlockTrace, http.Header, *jsonrpc.Error) {
	httpHeader := defaultExecutionHeader()

	declaredClasses, paidFeesOnL1, err := fetchDeclaredClassesAndL1Fees(
		transactions,
		classLookupState,
	)
	if err != nil {
		return nil, httpHeader, err
	}

	executionResult, vmErr := runner.Trace(
		transactions,
		declaredClasses,
		paidFeesOnL1,
		blockInfo,
		executionState,
		vm.TraceOptions{ReturnInitialReads: returnInitialReads},
	)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if vmErr != nil {
		if errors.Is(vmErr, throttler.ErrResourceBusy) {
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(vmErr.Error())
	}

	result, packErr := tracecache.FromVM(transactions, &executionResult, returnInitialReads)
	if packErr != nil {
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(packErr.Error())
	}
	return result, httpHeader, nil
}

// fetchDeclaredClassesAndL1Fees collects class declarations and L1Handler placeholder fees.
func fetchDeclaredClassesAndL1Fees(
	transactions []core.Transaction, state core.StateReader,
) ([]core.ClassDefinition, []*felt.Felt, *jsonrpc.Error) {
	var declaredClasses []core.ClassDefinition
	paidFeesOnL1 := []*felt.Felt{}

	for _, transaction := range transactions {
		switch tx := transaction.(type) {
		case *core.DeclareTransaction:
			class, stateErr := state.Class(tx.ClassHash)
			if stateErr != nil {
				return nil, nil, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
			}
			declaredClasses = append(declaredClasses, class.Class)
		case *core.L1HandlerTransaction:
			// TODO (granza): use real L1 message fee.
			paidFeesOnL1 = append(paidFeesOnL1, &felt.One)
		}
	}

	return declaredClasses, paidFeesOnL1, nil
}

/****************************************************
		Transaction Tracing Helpers
*****************************************************/

// findAndTraceFinalisedTransaction searches for a transaction in
// finalised blocks and returns its trace.
func (h *Handler) findAndTraceFinalisedTransaction(
	ctx context.Context, hash *felt.TransactionHash,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	blockNumber, txIndex, err := h.bcReader.BlockNumberAndIndexByTxHash(hash)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return TransactionTrace{}, nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	header, err := h.bcReader.BlockHeaderByNumber(blockNumber)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
		}
		return TransactionTrace{}, nil, rpccore.ErrInternal.CloneWithData(err)
	}

	blockTraces, httpHeader, rpcErr := h.traceFinalisedBlock(ctx, header, false)
	if rpcErr != nil {
		return TransactionTrace{}, nil, rpcErr
	}

	// txIndex comes from the tx-hash index while the traces come from a later read of the block, so
	// confirm the trace at that index really is the transaction that was asked for.
	if txIndex >= uint64(len(blockTraces.Traces)) ||
		!blockTraces.Traces[txIndex].Hash.Equal((*felt.Felt)(hash)) {
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	var trace TransactionTrace
	adaptCachedTrace(&blockTraces.Traces[txIndex], &trace)
	return trace, httpHeader, nil
}

// findAndTraceInPreConfirmed traces a transaction located in any block of the
// pre_confirmed chain (head+1 .. tip). The chain is scanned newest-first. The
// state immediately before txIndex is reconstructed by layering every chain
// entry's diff from chain bottom up to entry's block, then the entry's own
// transaction-level diffs up to (but not including) txIndex.
func (h *Handler) findAndTraceInPreConfirmed(
	hash *felt.TransactionHash,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	chain, err := h.syncReader.PreConfirmedChain()
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, pending.ErrPreConfirmedNotFound) {
			return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
		}
		return TransactionTrace{}, nil, rpccore.ErrInternal.CloneWithData(err)
	}

	for entry := range chain.NewestFirst() {
		transaction, transactionIndex, err := entry.TransactionByHash(hash)
		if err != nil {
			continue
		}

		state, baseCloser, err := chain.PreConfirmedStateBeforeIndexAt(
			entry.Block.Number,
			transactionIndex,
			h.bcReader,
		)
		if err != nil {
			return TransactionTrace{}, nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
		//nolint:gocritic // safe to defer in loop: we return on this iteration
		defer h.callAndLogErr(baseCloser, "Failed to close state in findAndTraceInPreConfirmed")

		blockInfo, rpcErr := h.buildBlockInfo(entry.GetHeader())
		if rpcErr != nil {
			return TransactionTrace{}, defaultExecutionHeader(), rpcErr
		}

		traces, httpHeader, rpcErr := traceTransactionsWithState(
			h.vm,
			[]core.Transaction{transaction},
			state, // execution state
			state, // class lookup state (same for preconfirmed)
			&blockInfo,
			false, // returnInitialReads
		)
		if rpcErr != nil {
			return TransactionTrace{}, httpHeader, rpcErr
		}
		var trace TransactionTrace
		adaptCachedTrace(&traces.Traces[0], &trace)
		return trace, httpHeader, nil
	}
	return TransactionTrace{}, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
}

/****************************************************
		Block Tracing Helpers
*****************************************************/

// traceFinalisedBlock caches local or feeder traces by block hash.
// See shouldFetchTracesFromFeederGateway for feeder trace edge cases.
func (h *Handler) traceFinalisedBlock(
	ctx context.Context,
	header *core.Header,
	returnInitialReads bool,
) (*tracecache.BlockTrace, http.Header, *jsonrpc.Error) {
	cached, lease, err := h.blockTraceCache.Acquire(
		ctx,
		header.Hash,
		func(b *tracecache.BlockTrace) bool {
			return b.Covers(returnInitialReads)
		},
	)
	if err != nil {
		return nil, defaultExecutionHeader(), rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}
	if lease == nil {
		return cached, defaultExecutionHeader(), nil
	}
	defer lease.Abort()

	fetchFromFeederGW, err := shouldFetchTracesFromFeederGateway(header, h.bcReader.Network())
	if err != nil {
		return nil,
			defaultExecutionHeader(),
			rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	if fetchFromFeederGW {
		traces, rpcErr := h.fetchTracesFromFeederGateway(ctx, header)
		if rpcErr != nil {
			return nil, defaultExecutionHeader(), rpcErr
		}

		lease.Publish(traces)
		return traces, defaultExecutionHeader(), nil
	}

	transactions, err := h.bcReader.TransactionsByBlockNumber(header.Number)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, defaultExecutionHeader(), rpccore.ErrBlockNotFound
		}

		return nil,
			defaultExecutionHeader(),
			rpccore.ErrInternal.CloneWithData(err)
	}

	response, httpHeader, rpcErr := h.traceBlockWithVM(header, transactions, returnInitialReads)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}
	lease.Publish(response)

	return response, httpHeader, nil
}

// traceBlockWithVM traces a block using the local VM.
func (h *Handler) traceBlockWithVM(
	header *core.Header,
	transactions []core.Transaction,
	returnInitialReads bool,
) (*tracecache.BlockTrace, http.Header, *jsonrpc.Error) {
	// Prepare execution state
	state, closer, err := h.bcReader.StateAtBlockHash(header.ParentHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, defaultExecutionHeader(), rpccore.ErrBlockNotFound
		}

		return nil,
			defaultExecutionHeader(),
			rpccore.ErrInternal.CloneWithData(err)
	}
	defer h.callAndLogErr(closer, "Failed to close state in traceBlockTransactions")

	// Get state to read class definitions for declare transactions
	var (
		headState       core.StateReader
		headStateCloser blockchain.StateCloser
	)

	headState, headStateCloser, err = h.bcReader.HeadState()
	if err != nil {
		return nil,
			defaultExecutionHeader(),
			jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	// Create block info
	blockInfo, rpcErr := h.buildBlockInfo(header)
	if rpcErr != nil {
		return nil, defaultExecutionHeader(), rpcErr
	}

	return traceTransactionsWithState(
		h.vm,
		transactions,
		state,
		headState,
		&blockInfo,
		returnInitialReads,
	)
}

// fetchTracesFromFeederGateway fetches block traces from the feeder gateway
// and fills in missing data.
func (h *Handler) fetchTracesFromFeederGateway(
	ctx context.Context, header *core.Header,
) (*tracecache.BlockTrace, *jsonrpc.Error) {
	if h.feederClient == nil {
		return nil, rpccore.ErrInternal.CloneWithData("no feeder client configured")
	}

	blockTrace, err := h.feederClient.BlockTrace(ctx, header.Hash.String())
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	transactions, receipts, err := h.bcReader.TransactionsAndReceiptsByBlockNumber(header.Number)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	kinds := make([]vm.TransactionType, len(transactions))
	for i, tx := range transactions {
		kinds[i] = vm.TransactionType(transactionTypeFrom(tx))
	}
	traces, err := tracecache.FromFeeder(kinds, receipts, &blockTrace)
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	return traces, nil
}

// buildBlockInfo builds block info for VM execution.
func (h *Handler) buildBlockInfo(header *core.Header) (vm.BlockInfo, *jsonrpc.Error) {
	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return vm.BlockInfo{}, rpccore.ErrInternal.CloneWithData(err)
	}

	return vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}, nil
}

// shouldFetchTracesFromFeederGateway determines if
// traces for a block should be fetched from the feeder gateway.
func shouldFetchTracesFromFeederGateway(
	header *core.Header,
	network *networks.Network,
) (bool, error) {
	blockVer, err := core.ParseBlockVersion(header.ProtocolVersion)
	if err != nil {
		return false, err
	}

	// We rely on the feeder gateway for Starknet version strictly older than "0.13.1.1"
	fetchFromFeederGW := blockVer.LessThan(core.Ver0_13_2) &&
		header.ProtocolVersion != "0.13.1.1"
	// This specific block range caused a re-org, also related with Cairo 0 and we have to
	// depend on the Sequencer to provide the correct traces
	fetchFromFeederGW = fetchFromFeederGW ||
		(header.Number >= 1943705 &&
			header.Number <= 1952704 &&
			*network == networks.Mainnet)

	return fetchFromFeederGW, nil
}

// defaultExecutionHeader returns a default HTTP header for execution responses,
// with the execution steps header set to "0".
func defaultExecutionHeader() http.Header {
	header := http.Header{}
	header.Set(ExecutionStepsHeader, "0")
	return header
}
