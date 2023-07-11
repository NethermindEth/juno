package rpc

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type TransactionTrace struct {
	// todo add the rest of types
	DeclareTxnTrace
}

type DeclareTxnTrace struct {
	ValidateInvocation    FunctionInvocation `json:"validate_invocation"`
	FeeTransferInvocation FunctionInvocation `json:"fee_transfer_invocation"`
}

type InvokeTxnTrace struct {
	ValidateInvocation FunctionInvocation `json:"validate_invocation"`
	// todo add failure case https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_trace_api_openrpc.json#L171
	ExecuteInvocation     FunctionInvocation `json:"execute_invocation"`
	FeeTransferInvocation FunctionInvocation `json:"fee_transfer_invocation"`
}

type FunctionInvocation struct {
	FunctionCall
	CallerAddress  *felt.Felt           `json:"caller_address"`
	ClassHash      *felt.Felt           `json:"class_hash"`
	EntryPointType EntryPointType       `json:"entry_point_type"`
	CallType       CallType             `json:"call_type"`
	Result         []felt.Felt          `json:"result"`
	Calls          []FunctionInvocation `json:"calls"`
	Events         []Event              `json:"events"`
	Messages       []MsgToL1            `json:"msg_to_l1"`
}

// enum: EXTERNAL, L1_HANDLER, CONSTRUCTOR
type EntryPointType string

// enum: LIBRARY_CALL, CALL
type CallType string

func adaptCallInfo(info *vm.CallInfo) FunctionInvocation {
	var result FunctionInvocation
	if info == nil {
		return result
	}

	result.FunctionCall = FunctionCall{
		// todo fill ContractAddress
		EntryPointSelector: info.Call.EntryPointSelector,
		Calldata:           info.Call.Calldata,
	}
	result.ClassHash = info.Call.ClassHash
	result.EntryPointType = EntryPointType(info.Call.EntryPointType)
	result.CallType = CallType(info.Call.CallType)
	result.Result = info.Execution.Retdata
	result.Calls = utils.Map(info.InnerCalls, func(innerInfo vm.CallInfo) FunctionInvocation {
		return adaptCallInfo(&innerInfo)
	})
	result.Events = utils.Map(info.Execution.Events, func(event vm.OrderedEvent) Event {
		e := event.Event
		return Event{
			Keys: utils.Map(e.Keys, utils.Ptr[felt.Felt]),
			Data: utils.Map(e.Data, utils.Ptr[felt.Felt]),
		}
	})
	result.Messages = utils.Map(info.Execution.L2ToL1Messages, func(msg vm.OrderedL2ToL1Message) MsgToL1 {
		m := msg.Message
		return MsgToL1{
			To:      m.ToAddress,
			Payload: utils.Map(m.Payload, utils.Ptr[felt.Felt]),
		}
	})

	return result
}
