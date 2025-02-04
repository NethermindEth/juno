package rpcv7

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpc_common"
	"github.com/NethermindEth/juno/utils"
)

type FeeUnit byte

const (
	WEI FeeUnit = iota
	FRI
)

func (u FeeUnit) MarshalText() ([]byte, error) {
	switch u {
	case WEI:
		return []byte("WEI"), nil
	case FRI:
		return []byte("FRI"), nil
	default:
		return nil, fmt.Errorf("unknown FeeUnit %v", u)
	}
}

type FeeEstimateV0_7 struct {
	GasConsumed     *felt.Felt `json:"gas_consumed"`
	GasPrice        *felt.Felt `json:"gas_price"`
	DataGasConsumed *felt.Felt `json:"data_gas_consumed"`
	DataGasPrice    *felt.Felt `json:"data_gas_price"`
	OverallFee      *felt.Felt `json:"overall_fee"`
	Unit            *FeeUnit   `json:"unit,omitempty"`
}

type FeeEstimate struct {
	L1GasConsumed     *felt.Felt `json:"l1_gas_consumed,omitempty"`
	L1GasPrice        *felt.Felt `json:"l1_gas_price,omitempty"`
	L2GasConsumed     *felt.Felt `json:"l2_gas_consumed,omitempty"`
	L2GasPrice        *felt.Felt `json:"l2_gas_price,omitempty"`
	L1DataGasConsumed *felt.Felt `json:"l1_data_gas_consumed,omitempty"`
	L1DataGasPrice    *felt.Felt `json:"l1_data_gas_price,omitempty"`
	OverallFee        *felt.Felt `json:"overall_fee"`
	Unit              *FeeUnit   `json:"unit,omitempty"`
}

/****************************************************
		Estimate Fee Handlers
*****************************************************/

func feeEstimateToV0_7(feeEstimate FeeEstimate) FeeEstimateV0_7 {
	return FeeEstimateV0_7{
		GasConsumed:     feeEstimate.L1GasConsumed,
		GasPrice:        feeEstimate.L1GasPrice,
		DataGasConsumed: feeEstimate.L1DataGasConsumed,
		DataGasPrice:    feeEstimate.L1DataGasPrice,
		OverallFee:      feeEstimate.OverallFee,
		Unit:            feeEstimate.Unit,
	}
}

func (h *Handler) EstimateFeeV0_7(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimateV0_7, http.Header, *jsonrpc.Error) {
	result, httpHeader, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), true)
	if err != nil {
		return nil, httpHeader, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimateV0_7 {
		return feeEstimateToV0_7(tx.FeeEstimation)
	}), httpHeader, nil
}

func (h *Handler) EstimateFee(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	result, httpHeader, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), true)
	if err != nil {
		return nil, httpHeader, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), httpHeader, nil
}

//nolint:gocritic
func (h *Handler) EstimateMessageFeeV0_7(msg MsgFromL1, id BlockID) (*FeeEstimateV0_7, http.Header, *jsonrpc.Error) {
	estimate, header, err := estimateMessageFee(msg, id, h.EstimateFee)
	if err != nil {
		return nil, header, err
	}
	estimateV0_7 := feeEstimateToV0_7(*estimate)
	return &estimateV0_7, header, nil
}

//nolint:gocritic
func (h *Handler) EstimateMessageFee(msg MsgFromL1, id BlockID) (*FeeEstimate, http.Header, *jsonrpc.Error) {
	return estimateMessageFee(msg, id, h.EstimateFee)
}

type estimateFeeHandler func(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error)

//nolint:gocritic
func estimateMessageFee(msg MsgFromL1, id BlockID, f estimateFeeHandler) (*FeeEstimate, http.Header, *jsonrpc.Error) {
	calldata := make([]*felt.Felt, 0, len(msg.Payload)+1)
	// The order of the calldata parameters matters. msg.From must be prepended.
	calldata = append(calldata, new(felt.Felt).SetBytes(msg.From.Bytes()))
	for payloadIdx := range msg.Payload {
		calldata = append(calldata, &msg.Payload[payloadIdx])
	}
	tx := BroadcastedTransaction{
		Transaction: Transaction{
			Type:               TxnL1Handler,
			ContractAddress:    &msg.To,
			EntryPointSelector: &msg.Selector,
			CallData:           &calldata,
			Version:            &felt.Zero, // Needed for transaction hash calculation.
			Nonce:              &felt.Zero, // Needed for transaction hash calculation.
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: new(felt.Felt).SetUint64(1),
	}
	estimates, httpHeader, rpcErr := f([]BroadcastedTransaction{tx}, nil, id)
	if rpcErr != nil {
		if rpcErr.Code == rpc_common.ErrTransactionExecutionError.Code {
			data := rpcErr.Data.(TransactionExecutionErrorData)
			return nil, httpHeader, makeContractError(errors.New(data.ExecutionError))
		}
		return nil, httpHeader, rpcErr
	}
	return &estimates[0], httpHeader, nil
}

type ContractErrorData struct {
	RevertError string `json:"revert_error"`
}

func makeContractError(err error) *jsonrpc.Error {
	return rpc_common.ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err.Error(),
	})
}
