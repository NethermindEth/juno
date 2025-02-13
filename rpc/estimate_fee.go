package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
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

type FeeEstimate struct {
	L1GasConsumed     *felt.Felt `json:"l1_gas_consumed,omitempty"`
	L1GasPrice        *felt.Felt `json:"l1_gas_price,omitempty"`
	L2GasConsumed     *felt.Felt `json:"l2_gas_consumed,omitempty"`
	L2GasPrice        *felt.Felt `json:"l2_gas_price,omitempty"`
	L1DataGasConsumed *felt.Felt `json:"l1_data_gas_consumed,omitempty"`
	L1DataGasPrice    *felt.Felt `json:"l1_data_gas_price,omitempty"`
	OverallFee        *felt.Felt `json:"overall_fee"`
	Unit              *FeeUnit   `json:"unit,omitempty"`
	rpcVersion        version
}

func (f FeeEstimate) MarshalJSON() ([]byte, error) {
	switch f.rpcVersion {
	case V0_7:
		return json.Marshal(struct {
			GasConsumed     *felt.Felt `json:"gas_consumed"`
			GasPrice        *felt.Felt `json:"gas_price"`
			DataGasConsumed *felt.Felt `json:"data_gas_consumed"`
			DataGasPrice    *felt.Felt `json:"data_gas_price"`
			OverallFee      *felt.Felt `json:"overall_fee"`
			Unit            *FeeUnit   `json:"unit,omitempty"`
		}{
			GasConsumed:     f.L1GasConsumed,
			GasPrice:        f.L1GasPrice,
			DataGasConsumed: f.L1DataGasConsumed,
			DataGasPrice:    f.L1DataGasPrice,
			OverallFee:      f.OverallFee,
			Unit:            f.Unit,
		})
	case V0_8:
		type alias FeeEstimate // avoid infinite recursion
		return json.Marshal(alias(f))
	default:
		return nil, errors.New("unknown FeeEstimate version")
	}
}

/****************************************************
		Estimate Fee Handlers
*****************************************************/

func (h *Handler) EstimateFeeV0_7(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	return h.estimateFee(broadcastedTxns, simulationFlags, id, V0_7)
}

func (h *Handler) EstimateFee(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	return h.estimateFee(broadcastedTxns, simulationFlags, id, V0_8)
}

func (h *Handler) estimateFee(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID, rpcVersion version,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	result, httpHeader, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), true, rpcVersion)
	if err != nil {
		return nil, httpHeader, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), httpHeader, nil
}

//nolint:gocritic
func (h *Handler) EstimateMessageFeeV0_7(msg MsgFromL1, id BlockID) (*FeeEstimate, http.Header, *jsonrpc.Error) {
	return estimateMessageFee(msg, id, h.EstimateFeeV0_7)
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
		if rpcErr.Code == ErrTransactionExecutionError.Code {
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
	return ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err.Error(),
	})
}
