package rpc

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

type TransactionTrace struct {
	Type                  TransactionType     `json:"type,omitempty"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     *ExecuteInvocation  `json:"execute_invocation,omitempty"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
	ConstructorInvocation *FunctionInvocation `json:"constructor_invocation,omitempty"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation,omitempty"`
	StateDiff             *StateDiff          `json:"state_diff,omitempty"`
}

type FunctionInvocation struct {
	ContractAddress    felt.Felt              `json:"contract_address"`
	EntryPointSelector *felt.Felt             `json:"entry_point_selector,omitempty"`
	Calldata           []felt.Felt            `json:"calldata"`
	CallerAddress      felt.Felt              `json:"caller_address"`
	ClassHash          *felt.Felt             `json:"class_hash,omitempty"`
	EntryPointType     string                 `json:"entry_point_type,omitempty"`
	CallType           string                 `json:"call_type,omitempty"`
	Result             []felt.Felt            `json:"result"`
	Calls              []FunctionInvocation   `json:"calls"`
	Events             []OrderedEvent         `json:"events"`
	Messages           []OrderedL2toL1Message `json:"messages"`
}

type ExecuteInvocation struct {
	RevertReason        string `json:"revert_reason,omitempty"`
	*FunctionInvocation `json:",omitempty"`
}

type OrderedEvent struct {
	Order *uint64 `json:"order,omitempty"`
	Event
}

type OrderedL2toL1Message struct {
	Order *uint64 `json:"order,omitempty"`
	MsgToL1
}

func adaptBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace, legacyTrace bool) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}
	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}
	traces := make([]TracedBlockTransaction, 0, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]
		trace := TransactionTrace{}
		if !legacyTrace {
			trace.Type = block.Transactions[index].Type
		}

		trace.FeeTransferInvocation = adaptFunctionInvocation(feederTrace.FeeTransferInvocation, legacyTrace)
		trace.ValidateInvocation = adaptFunctionInvocation(feederTrace.ValidateInvocation, legacyTrace)

		fnInvocation := adaptFunctionInvocation(feederTrace.FunctionInvocation, legacyTrace)
		switch block.Transactions[index].Type {
		case TxnDeploy:
			trace.ConstructorInvocation = fnInvocation
		case TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.ExecuteInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.ExecuteInvocation.FunctionInvocation = fnInvocation
			}
		case TxnL1Handler:
			trace.FunctionInvocation = fnInvocation
		}

		traceJSON, err := json.Marshal(trace)
		if err != nil {
			return nil, err
		}

		traces = append(traces, TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       traceJSON,
		})
	}
	return traces, nil
}

func adaptFunctionInvocation(snFnInvocation *starknet.FunctionInvocation, legacyTrace bool) *FunctionInvocation {
	if snFnInvocation == nil {
		return nil
	}

	orderPtr := func(o uint64) *uint64 {
		if legacyTrace {
			return nil
		}
		return &o
	}

	fnInvocation := FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              make([]FunctionInvocation, 0, len(snFnInvocation.InternalCalls)),
		Events:             make([]OrderedEvent, 0, len(snFnInvocation.Events)),
		Messages:           make([]OrderedL2toL1Message, 0, len(snFnInvocation.Messages)),
	}
	for index := range snFnInvocation.InternalCalls {
		fnInvocation.Calls = append(fnInvocation.Calls, *adaptFunctionInvocation(&snFnInvocation.InternalCalls[index], legacyTrace))
	}
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]
		fnInvocation.Events = append(fnInvocation.Events, OrderedEvent{
			Order: orderPtr(snEvent.Order),
			Event: Event{
				Keys: utils.Map(snEvent.Keys, utils.Ptr[felt.Felt]),
				Data: utils.Map(snEvent.Data, utils.Ptr[felt.Felt]),
			},
		})
	}
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]
		fnInvocation.Messages = append(fnInvocation.Messages, OrderedL2toL1Message{
			Order: orderPtr(snMessage.Order),
			MsgToL1: MsgToL1{
				Payload: utils.Map(snMessage.Payload, utils.Ptr[felt.Felt]),
				To:      common.HexToAddress(snMessage.ToAddr),
			},
		})
	}

	return &fnInvocation
}
