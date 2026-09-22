package rpc2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1/eth"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
)

func adaptReceipt(receipt *rpcv10.TransactionReceipt) (core.TransactionReceipt, error) {
	if missingField := missingReceiptField(receipt); missingField != "" {
		return core.TransactionReceipt{}, fmt.Errorf("receipt is missing %s", missingField)
	}

	return core.TransactionReceipt{
		Fee:                receipt.ActualFee.Amount,
		FeeUnit:            adaptFeeUnit(receipt.ActualFee.Unit),
		Events:             mapToPointers(receipt.Events, adaptEvent),
		TransactionHash:    receipt.Hash,
		L1ToL2Message:      nil,
		L2ToL1Message:      mapToPointers(receipt.MessagesSent, adaptMessageSent),
		Reverted:           receipt.ExecutionStatus == rpcv10.TxnFailure,
		RevertReason:       receipt.RevertReason,
		ExecutionResources: new(adaptExecutionResources(&receipt.ExecutionResources)),
	}, nil
}

func adaptFeeUnit(unit rpcv10.FeeUnit) core.FeeUnit {
	if unit == rpcv10.FRI {
		return core.STRK
	}
	return core.WEI
}

func adaptExecutionResources(resources *rpcv10.ExecutionResources) core.ExecutionResources {
	var adapted core.ExecutionResources
	adapted.TotalGasConsumed = &core.GasConsumed{
		L1Gas:     resources.L1Gas,
		L1DataGas: resources.L1DataGas,
		L2Gas:     resources.L2Gas,
	}
	return adapted
}

func adaptEvent(event *rpcv10.Event) core.Event {
	return core.Event(*event)
}

func adaptMessageSent(message rpcv10.MsgToL1) core.L2ToL1Message {
	to := message.To.Bytes()
	return core.L2ToL1Message{
		From:    message.From,
		Payload: message.Payload,
		To:      eth.AddressFromBytes(to[:]),
	}
}

func missingReceiptField(receipt *rpcv10.TransactionReceipt) string {
	if receipt.ActualFee.Amount == nil {
		return "actual_fee.amount"
	}

	for i, event := range receipt.Events {
		if event == nil {
			return fmt.Sprintf("events[%d]", i)
		}
		if event.From == nil {
			return fmt.Sprintf("events[%d].from_address", i)
		}
	}

	for i := range receipt.MessagesSent {
		if receipt.MessagesSent[i].From == nil {
			return fmt.Sprintf("messages_sent[%d].from_address", i)
		}
	}

	return ""
}

func mapToPointers[T, U any](values []T, adapt func(T) U) []*U {
	adapted := make([]*U, len(values))
	for i, value := range values {
		adapted[i] = new(adapt(value))
	}
	return adapted
}
