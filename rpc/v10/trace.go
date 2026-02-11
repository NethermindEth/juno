package rpcv10

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
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/sync/pendingdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type TransactionTrace struct {
	Type                  rpcv9.TransactionType     `json:"type"`
	ValidateInvocation    *rpcv9.FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *rpcv9.ExecuteInvocation  `json:"execute_invocation,omitempty" validate:"required_if=Type INVOKE"` //nolint:lll // struct tag exceeds line limit
	FeeTransferInvocation *rpcv9.FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *rpcv9.FunctionInvocation `json:"constructor_invocation,omitempty" validate:"required_if=Type DEPLOY_ACCOUNT"` //nolint:lll // struct tag exceeds line limit
	FunctionInvocation    *rpcv9.ExecuteInvocation  `json:"function_invocation,omitempty" validate:"required_if=Type L1_HANDLER"`        //nolint:lll // struct tag exceeds line limit
	StateDiff             *StateDiff                `json:"state_diff,omitempty"`
	ExecutionResources    *rpcv9.ExecutionResources `json:"execution_resources"`
}

/****************************************************
		Public API Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L11 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
func (h *Handler) TraceTransaction(
	ctx context.Context, hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	httpHeader := defaultExecutionHeader()

	if trace, header, err := h.findAndTraceFinalisedTransaction(ctx, hash); err == nil {
		return trace, header, nil
	} else if err != rpccore.ErrTxnHashNotFound {
		return TransactionTrace{}, httpHeader, rpccore.ErrTxnHashNotFound
	}

	// Try to find and trace transaction in pending data
	trace, header, err := h.findAndTraceInPendingData(hash)
	if err != nil {
		return TransactionTrace{}, httpHeader, err
	}
	return trace, header, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/39553a2e5216b7b5e06f6d44368317c0ccd79dfa/api/starknet_api_openrpc.json#L569
func (h *Handler) Call(
	funcCall *rpcv9.FunctionCall,
	id *rpcv9.BlockID,
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
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, rpcv9.MakeContractError(json.RawMessage(err.Error()))
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
		return nil, rpcv9.MakeContractError(json.RawMessage(strErr))
	}
	return res.Result, nil
}

// TraceBlockTransactions returns the trace for a given blockID
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L108 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
func (h *Handler) TraceBlockTransactions(
	ctx context.Context, id *rpcv9.BlockID, traceFlags []TraceFlag,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	if id.IsPreConfirmed() {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpccore.ErrCallOnPreConfirmed
	}

	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
	}

	returnInitialReads := slices.Contains(traceFlags, TraceReturnInitialReadsFlag)
	return h.traceBlockTransactions(ctx, block, returnInitialReads)
}

/****************************************************
		Core Tracing Logic
*****************************************************/

// traceTransactionsWithState traces a set of transactions using the provided VM and state readers.
//
// Parameters:
//
//   - vm: The virtual machine used for execution
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
	vm vm.VM,
	transactions []core.Transaction,
	executionState core.StateReader,
	classLookupState core.StateReader,
	blockInfo *vm.BlockInfo,
	returnInitialReads bool,
) ([]TracedBlockTransaction, *vm.InitialReads, http.Header, *jsonrpc.Error) {
	httpHeader := defaultExecutionHeader()

	// Collect declared classes and L1 fees for VM execution
	declaredClasses, paidFeesOnL1, err := fetchDeclaredClassesAndL1Fees(
		transactions,
		classLookupState,
	)
	if err != nil {
		return nil, nil, httpHeader, err
	}

	executionResult, vmErr := vm.Execute(
		transactions,
		declaredClasses,
		paidFeesOnL1,
		blockInfo,
		executionState,
		false, // skipValidate
		false, // skipFeeCharge
		false, // skipNonceCharge
		true,  // allowZeroMaxFee
		false, // allowNoSignature
		false, // isEstimateFee
		returnInitialReads,
	)

	httpHeader.Set(rpcv9.ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if vmErr != nil {
		if errors.Is(vmErr, utils.ErrResourceBusy) {
			return nil, nil, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(vmErr.Error())
	}

	// Adapt traces
	traces := make([]TracedBlockTransaction, len(executionResult.Traces))
	for index := range executionResult.Traces {
		// Adapt vm transaction trace to rpc v9 trace and add root level execution resources
		trace := AdaptVMTransactionTrace(&executionResult.Traces[index])

		trace.ExecutionResources = &rpcv9.ExecutionResources{
			InnerExecutionResources: rpcv9.InnerExecutionResources{
				L1Gas: executionResult.GasConsumed[index].L1Gas,
				L2Gas: executionResult.GasConsumed[index].L2Gas,
			},
			L1DataGas: executionResult.GasConsumed[index].L1DataGas,
		}

		traces[index] = TracedBlockTransaction{
			TraceRoot:       &trace,
			TransactionHash: transactions[index].Hash(),
		}
	}

	return traces, executionResult.InitialReads, httpHeader, nil
}

// fetchDeclaredClassesAndL1Fees collects class declarations and L1 handler fees for VM execution.
//
// Parameters:
//   - transactions: The transactions to process
//   - state: The state reader used to look up class definitions
//
// Returns the list of declared classes, L1 handler fees, and an error if any.
func fetchDeclaredClassesAndL1Fees(
	transactions []core.Transaction, state core.StateReader,
) ([]core.ClassDefinition, []*felt.Felt, *jsonrpc.Error) {
	var declaredClasses []core.ClassDefinition
	l1HandlerFees := []*felt.Felt{}

	for _, transaction := range transactions {
		switch tx := transaction.(type) {
		case *core.DeclareTransaction:
			class, stateErr := state.Class(tx.ClassHash)
			if stateErr != nil {
				return nil, nil, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
			}
			declaredClasses = append(declaredClasses, class.Class)
		case *core.L1HandlerTransaction:
			l1HandlerFees = append(l1HandlerFees, &felt.One)
		}
	}

	return declaredClasses, l1HandlerFees, nil
}

/****************************************************
		Transaction Tracing Helpers
*****************************************************/

// findAndTraceFinalisedTransaction searches for a transaction in
// finalised blocks and returns its trace.
func (h *Handler) findAndTraceFinalisedTransaction(
	ctx context.Context, hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	_, blockHash, _, err := h.bcReader.Receipt(hash)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return TransactionTrace{}, nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	block, err := h.bcReader.BlockByHash(blockHash)
	if err != nil {
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	txIndex, rpcErr := findTransactionInBlock(block, hash)
	if rpcErr != nil {
		return TransactionTrace{}, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
	}

	blockTracesResp, httpHeader, rpcErr := h.traceBlockTransactions(ctx, block, false)
	if rpcErr != nil {
		return TransactionTrace{}, nil, rpcErr
	}

	blockTraces := blockTracesResp.Traces
	return *blockTraces[txIndex].TraceRoot, httpHeader, nil
}

// findAndTraceInPendingData searches for a transaction across all pending data sources.
//
// This function searches in the following order:
// 1. Main pending block (can be pending or preconfirmed based on protocol version)
// 2. Prelatest block (if available when protocol version is >= 0.14.0)
//
// Returns ErrTxnHashNotFound if the transaction is not found in any pending data source.
func (h *Handler) findAndTraceInPendingData(
	hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	pendingData, err := h.PendingData()
	if err != nil {
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	if trace, header, err := h.findAndTraceInPendingBlock(pendingData, hash); err == nil {
		return trace, header, nil
	} else if err != rpccore.ErrTxnHashNotFound {
		return TransactionTrace{}, nil, err
	}
	return h.findAndTraceInPrelatestBlock(pendingData, hash)
}

// findAndTraceInPendingBlock finds and traces a transaction in the pending/pre_confirmed block.
func (h *Handler) findAndTraceInPendingBlock(
	pendingData core.PendingData, hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	block := pendingData.GetBlock()
	txIndex, rpcErr := findTransactionInBlock(block, hash)
	if rpcErr != nil {
		return TransactionTrace{}, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
	}

	switch pendingData.Variant() {
	case core.PreConfirmedBlockVariant:
		return h.traceInPreConfirmedBlock(pendingData, txIndex)
	case core.PendingBlockVariant:
		// TODO: remove pending when its will be no longer supported
		blockTracesResp, httpHeader, rpcErr := h.traceBlockWithVM(block, false)
		if rpcErr != nil {
			return TransactionTrace{}, nil, rpcErr
		}
		blockTraces := blockTracesResp.Traces
		return *blockTraces[txIndex].TraceRoot, httpHeader, nil
	default:
		// Unknown variant - this should not happen in normal operation
		return TransactionTrace{}, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
	}
}

// findAndTraceInPrelatestBlock finds and traces a transaction in the prelatest block.
func (h *Handler) findAndTraceInPrelatestBlock(
	pendingData core.PendingData, hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	preLatest := pendingData.GetPreLatest()
	if preLatest == nil {
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	txIndex, err := findTransactionInBlock(preLatest.Block, hash)
	if err != nil {
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	return h.traceInPrelatestBlock(preLatest, txIndex)
}

// traceInPrelatestBlock traces a transaction in the prelatest block.
func (h *Handler) traceInPrelatestBlock(
	preLatest *core.PreLatest, txIndex uint,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	state, closer, err := h.bcReader.StateAtBlockHash(preLatest.Block.ParentHash)
	if err != nil {
		return TransactionTrace{}, defaultExecutionHeader(), rpccore.ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in tracePreLatestTransaction")

	preLatestState := core.NewPendingState(
		preLatest.StateUpdate.StateDiff,
		preLatest.NewClasses,
		state,
	)

	blockInfo, rpcErr := h.buildBlockInfo(preLatest.Block.Header)
	if rpcErr != nil {
		return TransactionTrace{}, defaultExecutionHeader(), rpcErr
	}

	traces, _, httpHeader, rpcErr := traceTransactionsWithState(
		h.vm,
		preLatest.Block.Transactions,
		state,
		preLatestState,
		&blockInfo,
		false, // returnInitialReads
	)
	if rpcErr != nil {
		return TransactionTrace{}, httpHeader, rpcErr
	}

	return *traces[txIndex].TraceRoot, httpHeader, nil
}

// traceInPreConfirmedBlock traces a transaction in a preconfirmed block.
func (h *Handler) traceInPreConfirmedBlock(
	pendingData core.PendingData, txIndex uint,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	state, stateCloser, err := pendingdata.PendingStateBeforeIndex(pendingData, h.bcReader, txIndex)
	if err != nil {
		return TransactionTrace{}, nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(stateCloser, "Failed to close state in tracePreConfirmedTransaction")

	transaction := pendingData.GetBlock().Transactions[txIndex]

	blockInfo, rpcErr := h.buildBlockInfo(pendingData.GetHeader())
	if rpcErr != nil {
		return TransactionTrace{}, defaultExecutionHeader(), rpcErr
	}

	traces, _, httpHeader, rpcErr := traceTransactionsWithState(
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

	return *traces[0].TraceRoot, httpHeader, nil
}

/****************************************************
		Block Tracing Helpers
*****************************************************/

// traceBlockTransactions gets the trace for a block. The block will always be traced locally except
// on specific case such as with Starknet version 0.13.2 or lower or when it is certain range
func (h *Handler) traceBlockTransactions(
	ctx context.Context, block *core.Block, returnInitialReads bool,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	// Check if it was already traced
	cacheKey := rpccore.TraceCacheKey{BlockHash: *block.Hash}
	cachedResponse, hit := h.blockTraceCache.Get(cacheKey)
	if hit {
		if returnInitialReads {
			if cachedResponse.InitialReads != nil {
				return cachedResponse, defaultExecutionHeader(), nil
			}
			return TraceBlockTransactionsResponse{
				Traces:       cachedResponse.Traces,
				InitialReads: &InitialReads{},
			}, defaultExecutionHeader(), nil
		} else {
			return TraceBlockTransactionsResponse{
				Traces:       cachedResponse.Traces,
				InitialReads: nil,
			}, defaultExecutionHeader(), nil
		}
	}

	fetchFromFeederGW, err := shouldFetchTracesFromFeederGateway(block, h.bcReader.Network())
	if err != nil {
		return TraceBlockTransactionsResponse{},
			defaultExecutionHeader(),
			rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	if fetchFromFeederGW {
		traces, err := h.fetchTracesFromFeederGateway(ctx, block)
		if err != nil {
			return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), err
		}
		// Feeder gateway doesn't provide initial reads, so cache without them
		h.blockTraceCache.Add(rpccore.TraceCacheKey{BlockHash: *block.Hash}, TraceBlockTransactionsResponse{
			Traces:       traces,
			InitialReads: nil,
		})

		return TraceBlockTransactionsResponse{
			Traces:       traces,
			InitialReads: &InitialReads{},
		}, defaultExecutionHeader(), nil
	}

	return h.traceBlockWithVM(block, returnInitialReads)
}

// traceBlockWithVM traces a block using the local VM and stores the result in the block cache.
func (h *Handler) traceBlockWithVM(block *core.Block, returnInitialReads bool) (
	TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error,
) {
	// Prepare execution state
	state, closer, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpccore.ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in traceBlockTransactions")

	// Get state to read class definitions for declare transactions
	var (
		headState       core.StateReader
		headStateCloser blockchain.StateCloser
	)
	// TODO: remove pending variant when it is no longer supported
	isPending := block.Hash == nil
	if isPending {
		headState, headStateCloser, err = h.PendingState()
	} else {
		headState, headStateCloser, err = h.bcReader.HeadState()
	}
	if err != nil {
		return TraceBlockTransactionsResponse{},
			defaultExecutionHeader(),
			jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	// Create block info
	blockInfo, rpcErr := h.buildBlockInfo(block.Header)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
	}

	traces, vmInitialReads, httpHeader, rpcErr := traceTransactionsWithState(
		h.vm,
		block.Transactions,
		state,
		headState,
		&blockInfo,
		returnInitialReads,
	)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, httpHeader, rpcErr
	}

	var adaptedInitialReads *InitialReads
	if vmInitialReads != nil && returnInitialReads {
		adapted := adaptVMInitialReads(vmInitialReads)
		adaptedInitialReads = &adapted
	}

	// Always cache result for finalised blocks with initial reads if we have them (they can be used for both requests)
	if !isPending {
		h.blockTraceCache.Add(rpccore.TraceCacheKey{BlockHash: *block.Hash}, TraceBlockTransactionsResponse{
			Traces:       traces,
			InitialReads: adaptedInitialReads,
		})
	}

	return TraceBlockTransactionsResponse{
		Traces:       traces,
		InitialReads: adaptedInitialReads,
	}, httpHeader, nil
}

// fetchTracesFromFeederGateway fetches block traces from the feeder gateway
// and fills in missing data.
func (h *Handler) fetchTracesFromFeederGateway(
	ctx context.Context, block *core.Block,
) ([]TracedBlockTransaction, *jsonrpc.Error) {
	if h.feederClient == nil {
		return nil, rpccore.ErrInternal.CloneWithData("no feeder client configured")
	}

	blockTrace, err := h.feederClient.BlockTrace(ctx, block.Hash.String())
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	traces, err := AdaptFeederBlockTrace(block, blockTrace)
	if err != nil {
		return nil, rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	traces = fillFeederGatewayData(traces, block.Receipts)

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
func shouldFetchTracesFromFeederGateway(block *core.Block, network *utils.Network) (bool, error) {
	blockVer, err := core.ParseBlockVersion(block.ProtocolVersion)
	if err != nil {
		return false, err
	}

	// We rely on the feeder gateway for Starknet version strictly older than "0.13.1.1"
	fetchFromFeederGW := blockVer.LessThan(core.Ver0_13_2) &&
		block.ProtocolVersion != "0.13.1.1"
	// This specific block range caused a re-org, also related with Cairo 0 and we have to
	// depend on the Sequencer to provide the correct traces
	fetchFromFeederGW = fetchFromFeederGW ||
		(block.Number >= 1943705 &&
			block.Number <= 1952704 &&
			*network == utils.Mainnet)

	return fetchFromFeederGW, nil
}

// fillFeederGatewayData mutates the `traces` argument and fills it with
// the data from `receipts` which the Feeder Gateway doesn't provide by default.
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

		traces[index].TraceRoot.ExecutionResources = &rpcv9.ExecutionResources{
			InnerExecutionResources: rpcv9.InnerExecutionResources{
				L1Gas: tgs.L1Gas,
				L2Gas: tgs.L2Gas,
			},
			L1DataGas: tgs.L1DataGas,
		}
	}

	return traces
}

// defaultExecutionHeader returns a default HTTP header for execution responses,
// with the execution steps header set to "0".
func defaultExecutionHeader() http.Header {
	header := http.Header{}
	header.Set(rpcv9.ExecutionStepsHeader, "0")
	return header
}

// findTransactionInBlock locates the index of a transaction with the given hash in a block.
//
// Returns the index of the transaction and nil if found, or 0 and ErrTxnHashNotFound if not found.
func findTransactionInBlock(block *core.Block, hash *felt.Felt) (uint, *jsonrpc.Error) {
	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(hash)
	})
	if txIndex == -1 {
		return 0, rpccore.ErrTxnHashNotFound
	}
	return uint(txIndex), nil
}
