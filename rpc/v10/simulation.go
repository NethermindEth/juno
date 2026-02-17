package rpcv10

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

const ExecutionStepsHeader string = "X-Cairo-Steps"

type SimulationFlag int

const (
	SkipValidateFlag SimulationFlag = iota + 1
	SkipFeeChargeFlag
	ReturnInitialReadsFlag
)

func (s *SimulationFlag) UnmarshalJSON(bytes []byte) (err error) {
	switch flag := string(bytes); flag {
	case `"SKIP_VALIDATE"`:
		*s = SkipValidateFlag
	case `"SKIP_FEE_CHARGE"`:
		*s = SkipFeeChargeFlag
	case `"RETURN_INITIAL_READS"`:
		*s = ReturnInitialReadsFlag
	default:
		err = fmt.Errorf("unknown simulation flag %q", flag)
	}

	return err
}

type TraceFlag int

const (
	TraceReturnInitialReadsFlag TraceFlag = iota + 1
)

func (t *TraceFlag) UnmarshalJSON(bytes []byte) (err error) {
	switch flag := string(bytes); flag {
	case `"RETURN_INITIAL_READS"`:
		*t = TraceReturnInitialReadsFlag
	default:
		err = fmt.Errorf("unknown trace flag %q", flag)
	}

	return err
}

type EstimateFlag int

const (
	EstimateSkipValidateFlag EstimateFlag = iota + 1
)

func (e *EstimateFlag) UnmarshalJSON(bytes []byte) (err error) {
	switch flag := string(bytes); flag {
	case `"SKIP_VALIDATE"`:
		*e = EstimateSkipValidateFlag
	default:
		err = fmt.Errorf("unknown estimate flag %q", flag)
	}

	return err
}

// ToSimulationFlag converts an EstimateFlag to the corresponding SimulationFlag.
func (e EstimateFlag) ToSimulationFlag() (SimulationFlag, error) {
	switch e {
	case EstimateSkipValidateFlag:
		return SkipValidateFlag, nil
	default:
		return 0, fmt.Errorf("unknown estimate flag %v", e)
	}
}

type SimulatedTransaction struct {
	TransactionTrace *TransactionTrace `json:"transaction_trace,omitempty"`
	FeeEstimation    rpcv9.FeeEstimate `json:"fee_estimation,omitzero"`
}

// SimulateTransactionsResponse represents the response for simulateTransactions.
// When RETURN_INITIAL_READS flag is not set, it marshals as an array.
// When RETURN_INITIAL_READS flag is set, it marshals as an object
// with simulated_transactions and initial_reads.
type SimulateTransactionsResponse struct {
	SimulatedTransactions []SimulatedTransaction `json:"simulated_transactions"`
	InitialReads          *InitialReads          `json:"initial_reads"`
}

func (r SimulateTransactionsResponse) MarshalJSON() ([]byte, error) {
	if r.InitialReads == nil {
		return json.Marshal(r.SimulatedTransactions)
	}
	type simulateTransactionsResponse SimulateTransactionsResponse
	response := simulateTransactionsResponse(r)
	return json.Marshal(response)
}

type TracedBlockTransaction struct {
	TraceRoot       *TransactionTrace `json:"trace_root,omitempty"`
	TransactionHash *felt.Felt        `json:"transaction_hash,omitempty"`
}

// TraceBlockTransactionsResponse represents the response for traceBlockTransactions.
// When RETURN_INITIAL_READS flag is not set, it marshals as an array.
// When RETURN_INITIAL_READS flag is set, it marshals as an object with traces and initial_reads.
type TraceBlockTransactionsResponse struct {
	Traces       []TracedBlockTransaction `json:"traces"`
	InitialReads *InitialReads            `json:"initial_reads"`
}

func (r TraceBlockTransactionsResponse) MarshalJSON() ([]byte, error) {
	if r.InitialReads == nil {
		return json.Marshal(r.Traces)
	}
	type traceBlockTransactionsResponse TraceBlockTransactionsResponse
	response := traceBlockTransactionsResponse(r)
	return json.Marshal(response)
}

type StorageEntry struct {
	ContractAddress felt.Address `json:"contract_address"`
	Key             felt.Felt    `json:"key"`
	Value           felt.Felt    `json:"value"`
}

type NonceEntry struct {
	ContractAddress felt.Address `json:"contract_address"`
	Nonce           felt.Felt    `json:"nonce"`
}

type ClassHashEntry struct {
	ContractAddress felt.Address   `json:"contract_address"`
	ClassHash       felt.ClassHash `json:"class_hash"`
}

type DeclaredContractEntry struct {
	ClassHash  felt.ClassHash `json:"class_hash"`
	IsDeclared bool           `json:"is_declared"`
}

type InitialReads struct {
	Storage           []StorageEntry          `json:"storage"`
	Nonces            []NonceEntry            `json:"nonces"`
	ClassHashes       []ClassHashEntry        `json:"class_hashes"`
	DeclaredContracts []DeclaredContractEntry `json:"declared_contracts"`
}

type BroadcastedTransactionInputs = rpccore.LimitSlice[
	BroadcastedTransaction,
	rpccore.SimulationLimit,
]

/****************************************************
		Simulate Handlers
*****************************************************/

func (h *Handler) SimulateTransactions(
	ctx context.Context,
	id *rpcv9.BlockID,
	transactions BroadcastedTransactionInputs,
	simulationFlags []SimulationFlag,
) (SimulateTransactionsResponse, http.Header, *jsonrpc.Error) {
	return h.simulateTransactions(ctx, id, transactions.Data, simulationFlags, false, false)
}

func (h *Handler) simulateTransactions(
	ctx context.Context,
	id *rpcv9.BlockID,
	transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag,
	errOnRevert bool,
	isEstimateFee bool,
) (SimulateTransactionsResponse, http.Header, *jsonrpc.Error) {
	skipFeeCharge := slices.Contains(simulationFlags, SkipFeeChargeFlag)
	skipValidate := slices.Contains(simulationFlags, SkipValidateFlag)
	returnInitialReads := slices.Contains(simulationFlags, ReturnInitialReadsFlag)

	httpHeader := http.Header{}
	httpHeader.Set(ExecutionStepsHeader, "0")

	state, closer, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return SimulateTransactionsResponse{}, httpHeader, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateFee")

	header, rpcErr := h.blockHeaderByID(id)
	if rpcErr != nil {
		return SimulateTransactionsResponse{}, httpHeader, rpcErr
	}

	network := h.bcReader.Network()
	txns, classes, paidFeesOnL1, rpcErr := h.prepareTransactions(
		ctx, transactions, network,
	)
	if rpcErr != nil {
		return SimulateTransactionsResponse{}, httpHeader, rpcErr
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return SimulateTransactionsResponse{}, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	executionResults, err := h.vm.Execute(
		txns,
		classes,
		paidFeesOnL1,
		&blockInfo,
		state,
		skipFeeCharge,
		skipValidate,
		errOnRevert,
		true,
		true,
		isEstimateFee,
		returnInitialReads,
	)
	if err != nil {
		return SimulateTransactionsResponse{}, httpHeader, handleExecutionError(err)
	}

	httpHeader.Set(ExecutionStepsHeader, strconv.FormatUint(executionResults.NumSteps, 10))

	simulatedTransactions, err := createSimulatedTransactions(&executionResults, txns, header)
	if err != nil {
		return SimulateTransactionsResponse{}, httpHeader, rpccore.ErrInternal.CloneWithData(err)
	}

	var adaptedInitialReads *InitialReads
	if executionResults.InitialReads != nil && returnInitialReads {
		adapted := adaptVMInitialReads(executionResults.InitialReads)
		adaptedInitialReads = &adapted
	}

	return SimulateTransactionsResponse{
		SimulatedTransactions: simulatedTransactions,
		InitialReads:          adaptedInitialReads,
	}, httpHeader, nil
}

func isVersion3(version *felt.Felt) bool {
	return version != nil && version.Equal(&rpcv6.RPCVersion3Value)
}

func checkTxHasSenderAddress(tx *BroadcastedTransaction) bool {
	return (tx.Transaction.Type == rpcv9.TxnDeclare ||
		tx.Transaction.Type == rpcv9.TxnInvoke) &&
		isVersion3(tx.Transaction.Version) &&
		tx.Transaction.SenderAddress == nil
}

func checkTxHasResourceBounds(tx *BroadcastedTransaction) bool {
	return (tx.Transaction.Type == rpcv9.TxnInvoke ||
		tx.Transaction.Type == rpcv9.TxnDeployAccount ||
		tx.Transaction.Type == rpcv9.TxnDeclare) &&
		isVersion3(tx.Transaction.Version) &&
		tx.Transaction.ResourceBounds == nil
}

func (h *Handler) prepareTransactions(
	ctx context.Context,
	transactions []BroadcastedTransaction,
	network *utils.Network,
) ([]core.Transaction, []core.ClassDefinition, []*felt.Felt, *jsonrpc.Error) {
	txns := make([]core.Transaction, len(transactions))
	var classes []core.ClassDefinition
	paidFeesOnL1 := make([]*felt.Felt, 0)

	for idx := range transactions {
		// Check for missing required fields in struct that can't be validated by
		// jsonschema due to validation happening after omit empty
		//
		// TODO: as its expected that this will happen in other cases as well,
		// it might be a good idea to implement a custom validator and unmarshal handler
		// to solve this problem in a more elegant way
		if checkTxHasSenderAddress(&transactions[idx]) {
			return nil, nil, nil, jsonrpc.Err(
				jsonrpc.InvalidParams,
				"sender_address is required for this transaction type",
			)
		}

		if checkTxHasResourceBounds(&transactions[idx]) {
			return nil, nil, nil, jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			)
		}

		txn, declaredClass, paidFeeOnL1, aErr := AdaptBroadcastedTransaction(
			ctx,
			h.compiler,
			&transactions[idx],
			network,
		)
		if aErr != nil {
			return nil, nil, nil, jsonrpc.Err(jsonrpc.InvalidParams, aErr.Error())
		}

		if paidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, paidFeeOnL1)
		}

		txns[idx] = txn
		if declaredClass != nil {
			classes = append(classes, declaredClass)
		}
	}

	return txns, classes, paidFeesOnL1, nil
}

func handleExecutionError(err error) *jsonrpc.Error {
	if errors.Is(err, utils.ErrResourceBusy) {
		return rpccore.ErrInternal.CloneWithData(rpccore.ThrottledVMErr)
	}
	var txnExecutionError vm.TransactionExecutionError
	if errors.As(err, &txnExecutionError) {
		return rpcv9.MakeTransactionExecutionError(&txnExecutionError)
	}
	return rpccore.ErrUnexpectedError.CloneWithData(err.Error())
}

func createSimulatedTransactions(
	executionResults *vm.ExecutionResults, txns []core.Transaction, header *core.Header,
) ([]SimulatedTransaction, error) {
	overallFees := executionResults.OverallFees
	traces := executionResults.Traces
	gasConsumed := executionResults.GasConsumed
	daGas := executionResults.DataAvailability

	if len(overallFees) != len(traces) || len(overallFees) != len(gasConsumed) ||
		len(overallFees) != len(daGas) || len(overallFees) != len(txns) {
		return nil, fmt.Errorf(
			"inconsistent lengths: "+
				"%d overall fees, %d traces, %d gas consumed, %d data availability, %d txns",
			len(overallFees),
			len(traces),
			len(gasConsumed),
			len(daGas),
			len(txns),
		)
	}

	l1GasPriceWei := header.L1GasPriceETH
	l1GasPriceStrk := header.L1GasPriceSTRK
	l2GasPriceWei := &felt.One
	l2GasPriceStrk := &felt.One
	l1DataGasPriceWei := &felt.One
	l1DataGasPriceStrk := &felt.One

	if gasPrice := header.L2GasPrice; gasPrice != nil {
		l2GasPriceWei = gasPrice.PriceInWei
		l2GasPriceStrk = gasPrice.PriceInFri
	}
	if gasPrice := header.L1DataGasPrice; gasPrice != nil {
		l1DataGasPriceWei = gasPrice.PriceInWei
		l1DataGasPriceStrk = gasPrice.PriceInFri
	}

	simulatedTransactions := make([]SimulatedTransaction, len(overallFees))
	for i, overallFee := range overallFees {
		// Adapt transaction trace to rpc v9 trace
		trace := utils.HeapPtr(AdaptVMTransactionTrace(&traces[i]))

		// Add root level execution resources
		trace.ExecutionResources = &rpcv9.ExecutionResources{
			InnerExecutionResources: rpcv9.InnerExecutionResources{
				L1Gas: gasConsumed[i].L1Gas,
				L2Gas: gasConsumed[i].L2Gas,
			},
			L1DataGas: gasConsumed[i].L1DataGas,
		}

		// Compute data for FeeEstimate
		var l1GasPrice, l2GasPrice, l1DataGasPrice *felt.Felt

		// estimateFee only allows FRI as unit
		// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L3586 //nolint:lll
		//
		//nolint:lll // URL exceeds line limit but should remain intact for reference
		feeUnit := rpcv9.FRI
		// estimateMessageFee only allow WEI as unit
		// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L3605 //nolint:lll
		//
		//nolint:lll // URL exceeds line limit but should remain intact for reference
		if traces[i].Type == vm.TxnL1Handler {
			feeUnit = rpcv9.WEI
		}

		switch feeUnit {
		case rpcv9.WEI:
			l1GasPrice = l1GasPriceWei
			l2GasPrice = l2GasPriceWei
			l1DataGasPrice = l1DataGasPriceWei
		case rpcv9.FRI:
			l1GasPrice = l1GasPriceStrk
			l2GasPrice = l2GasPriceStrk
			l1DataGasPrice = l1DataGasPriceStrk
		}

		simulatedTransactions[i] = SimulatedTransaction{
			TransactionTrace: trace,
			FeeEstimation: rpcv9.FeeEstimate{
				L1GasConsumed:     felt.NewFromUint64[felt.Felt](gasConsumed[i].L1Gas),
				L1GasPrice:        l1GasPrice,
				L2GasConsumed:     felt.NewFromUint64[felt.Felt](gasConsumed[i].L2Gas),
				L2GasPrice:        l2GasPrice,
				L1DataGasConsumed: felt.NewFromUint64[felt.Felt](gasConsumed[i].L1DataGas),
				L1DataGasPrice:    l1DataGasPrice,
				OverallFee:        overallFee,
				Unit:              &feeUnit,
			},
		}
	}
	return simulatedTransactions, nil
}
