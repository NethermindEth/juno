package rpcv9

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
	"github.com/NethermindEth/juno/sync/pendingdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

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
	EntryPointType     string                       `json:"entry_point_type"` // todo(rdr): use an enum here
	CallType           string                       `json:"call_type"`        // todo(rdr): use an enum here
	Result             []felt.Felt                  `json:"result"`
	Calls              []FunctionInvocation         `json:"calls"`
	Events             []rpcv6.OrderedEvent         `json:"events"`
	Messages           []rpcv6.OrderedL2toL1Message `json:"messages"`
	ExecutionResources *InnerExecutionResources     `json:"execution_resources"`
	IsReverted         bool                         `json:"is_reverted"`
}

/****************************************************
		Public API Handlers
*****************************************************/

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(
	ctx context.Context, hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	httpHeader := defaultExecutionHeader()

	// Try to find and trace transaction in finalised blocks
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

// TraceBlockTransactions returns the trace for a given blockID
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_trace_api_openrpc.json#L108 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
func (h *Handler) TraceBlockTransactions(
	ctx context.Context, id *BlockID,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	if id.IsPreConfirmed() {
		return nil, defaultExecutionHeader(), rpccore.ErrCallOnPreConfirmed
	}

	block, rpcErr := h.blockByID(id)
	if rpcErr != nil {
		return nil, defaultExecutionHeader(), rpcErr
	}

	return h.traceBlockTransactions(ctx, block)
}

// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L579 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
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
func traceTransactionsWithState(
	vm vm.VM,
	transactions []core.Transaction,
	executionState core.StateReader,
	classLookupState core.StateReader,
	blockInfo *vm.BlockInfo,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	httpHeader := defaultExecutionHeader()

	// Collect declared classes and L1 fees for VM execution
	declaredClasses, paidFeesOnL1, err := fetchDeclaredClassesAndL1Fees(
		transactions,
		classLookupState,
	)
	if err != nil {
		return nil, httpHeader, err
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
	)

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResult.NumSteps, 10))

	if vmErr != nil {
		if errors.Is(vmErr, utils.ErrResourceBusy) {
			return nil, httpHeader, rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
		}
		return nil, httpHeader, rpccore.ErrUnexpectedError.CloneWithData(vmErr.Error())
	}

	// Adapt traces
	traces := make([]TracedBlockTransaction, len(executionResult.Traces))
	for index := range executionResult.Traces {
		// Adapt vm transaction trace to rpc v9 trace and add root level execution resources
		trace := AdaptVMTransactionTrace(&executionResult.Traces[index])

		trace.ExecutionResources = &ExecutionResources{
			InnerExecutionResources: InnerExecutionResources{
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

	return traces, httpHeader, nil
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
) ([]core.Class, []*felt.Felt, *jsonrpc.Error) {
	var declaredClasses []core.Class
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
			var fee felt.Felt
			l1HandlerFees = append(l1HandlerFees, fee.SetUint64(1))
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

	blockTraces, httpHeader, rpcErr := h.traceBlockTransactions(ctx, block)
	if rpcErr != nil {
		return TransactionTrace{}, nil, rpcErr
	}

	return *blockTraces[txIndex].TraceRoot, httpHeader, nil
}

// TODO: Add support for prelatest block tracing.
// findAndTraceInPendingData searches for a transaction across all pending data sources.
//
// This function searches in the following order:
// 1. Main pending block (can be pending or preconfirmed based on protocol version)
//
// Returns ErrTxnHashNotFound if the transaction is not found in any pending data source.
func (h *Handler) findAndTraceInPendingData(
	hash *felt.Felt,
) (TransactionTrace, http.Header, *jsonrpc.Error) {
	pendingData, err := h.PendingData()
	if err != nil {
		return TransactionTrace{}, nil, rpccore.ErrTxnHashNotFound
	}

	return h.findAndTraceInPendingBlock(pendingData, hash)
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
		blockTraces, httpHeader, rpcErr := h.traceBlockWithVM(block)
		if rpcErr != nil {
			return TransactionTrace{}, nil, rpcErr
		}
		return *blockTraces[txIndex].TraceRoot, httpHeader, nil
	default:
		// Unknown variant - this should not happen in normal operation
		return TransactionTrace{}, defaultExecutionHeader(), rpccore.ErrTxnHashNotFound
	}
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

	traces, httpHeader, rpcErr := traceTransactionsWithState(
		h.vm,
		[]core.Transaction{transaction},
		state, // execution state
		state, // class lookup state (same for preconfirmed)
		&blockInfo,
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
	ctx context.Context, block *core.Block,
) ([]TracedBlockTransaction, http.Header, *jsonrpc.Error) {
	// Check if it was already traced
	traces, hit := h.blockTraceCache.Get(rpccore.TraceCacheKey{BlockHash: *block.Hash})
	if hit {
		return traces, defaultExecutionHeader(), nil
	}

	fetchFromFeederGW, err := h.shouldFetchTracesFromFeederGateway(block)
	if err != nil {
		return nil, defaultExecutionHeader(), rpccore.ErrUnexpectedError.CloneWithData(err.Error())
	}

	if fetchFromFeederGW {
		traces, err := h.fetchTracesFromFeederGateway(ctx, block)
		if err != nil {
			return nil, defaultExecutionHeader(), err
		}
		h.blockTraceCache.Add(rpccore.TraceCacheKey{BlockHash: *block.Hash}, traces)
		return traces, defaultExecutionHeader(), nil
	}

	return h.traceBlockWithVM(block)
}

// shouldFetchTracesFromFeederGateway determines if
// traces for a block should be fetched from the feeder gateway.
func (h *Handler) shouldFetchTracesFromFeederGateway(block *core.Block) (bool, error) {
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
			*h.bcReader.Network() == utils.Mainnet)

	return fetchFromFeederGW, nil
}

// traceBlockWithVM traces a block using the local VM and stores the result in the block cache.
func (h *Handler) traceBlockWithVM(block *core.Block) (
	[]TracedBlockTransaction, http.Header, *jsonrpc.Error,
) {
	// Prepare execution state
	state, closer, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return nil, defaultExecutionHeader(), rpccore.ErrBlockNotFound
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
		return nil, defaultExecutionHeader(), jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	// Create block info
	blockInfo, rpcErr := h.buildBlockInfo(block.Header)
	if rpcErr != nil {
		return nil, defaultExecutionHeader(), rpcErr
	}

	traces, httpHeader, rpcErr := traceTransactionsWithState(
		h.vm,
		block.Transactions,
		state,
		headState,
		&blockInfo,
	)
	if rpcErr != nil {
		return nil, httpHeader, rpcErr
	}

	// Cache result for finalised blocks
	if !isPending {
		h.blockTraceCache.Add(rpccore.TraceCacheKey{BlockHash: *block.Hash}, traces)
	}

	return traces, httpHeader, nil
}

// fetchTracesFromFeederGateway fetches block traces from the feeder gateway
// and fills in missing data.
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

// defaultExecutionHeader returns a default HTTP header for execution responses,
// with the execution steps header set to "0".
func defaultExecutionHeader() http.Header {
	header := http.Header{}
	header.Set(ExecutionStepsHeader, "0")
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
