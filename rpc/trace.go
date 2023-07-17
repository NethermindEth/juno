package rpc

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type TransactionTrace interface {
	trace()
}

type DeclareTxnTrace struct {
	ValidateInvocation    FunctionInvocation `json:"validate_invocation,omitempty"`
	FeeTransferInvocation FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
}

func (DeclareTxnTrace) trace() {}

type InvokeTxnTrace struct {
	ValidateInvocation    FunctionInvocation `json:"validate_invocation,omitempty"`
	ExecuteInvocation     ExecuteInvocation  `json:"execute_invocation,omitempty"`
	FeeTransferInvocation FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
}

func (InvokeTxnTrace) trace() {}

type DeployAccountTxnTrace struct {
	ValidateInvocation    FunctionInvocation `json:"validate_invocation,omitempty"`
	ConstructorInvocation FunctionInvocation `json:"constructor_invocation,omitempty"`
	FeeTransferInvocation FunctionInvocation `json:"fee_transfer_invocation,omitempty"`
}

func (DeployAccountTxnTrace) trace() {}

type L1HandlerTxnTrace struct {
	FunctionInvocation FunctionInvocation `json:"function_invocation,omitempty"`
}

func (L1HandlerTxnTrace) trace() {}

type ExecuteInvocation struct {
	FunctionInvocation
	RevertReason string `json:"revert_reason,omitempty"`
}

type FunctionInvocation struct {
	FunctionCall
	CallerAddress  *felt.Felt           `json:"caller_address,omitempty"`
	ClassHash      *felt.Felt           `json:"class_hash,omitempty"`
	EntryPointType EntryPointType       `json:"entry_point_type,omitempty"`
	CallType       CallType             `json:"call_type,omitempty"`
	Result         []felt.Felt          `json:"result,omitempty"`
	Calls          []FunctionInvocation `json:"calls,omitempty"`
	Events         []EventContent       `json:"events,omitempty"`
	Messages       []MsgToL1            `json:"msg_to_l1,omitempty"`
}

// enum: EXTERNAL, L1_HANDLER, CONSTRUCTOR
type EntryPointType string

// enum: LIBRARY_CALL, CALL
type CallType string

type EventContent struct {
	Keys []felt.Felt `json:"keys"`
	Data []felt.Felt `json:"data"`
}

func adaptTxExecutionInfo(tx core.Transaction, info vm.TransactionExecutionInfo) (TransactionTrace, *jsonrpc.Error) {
	switch tx.(type) {
	case *core.DeclareTransaction:
		return DeclareTxnTrace{
			ValidateInvocation:    adaptCallInfo(info.ValidateCallInfo),
			FeeTransferInvocation: adaptCallInfo(info.FeeTransferCallInfo),
		}, nil
	case *core.InvokeTransaction:
		return InvokeTxnTrace{
			ValidateInvocation:    adaptCallInfo(info.ValidateCallInfo),
			FeeTransferInvocation: adaptCallInfo(info.FeeTransferCallInfo),
			ExecuteInvocation: ExecuteInvocation{
				FunctionInvocation: adaptCallInfo(info.ExecuteCallInfo),
				// todo add revert info
			},
		}, nil
	case *core.DeployAccountTransaction:
		return DeployAccountTxnTrace{
			ValidateInvocation:    adaptCallInfo(info.ValidateCallInfo),
			FeeTransferInvocation: adaptCallInfo(info.FeeTransferCallInfo),
			// todo support constructor
			ConstructorInvocation: FunctionInvocation{},
		}, nil
	case *core.L1HandlerTransaction:
		return L1HandlerTxnTrace{
			// todo support it
			FunctionInvocation: FunctionInvocation{},
		}, nil
	default:
		msg := fmt.Sprintf("unknown transaction type %T", tx)
		return nil, jsonrpc.Err(jsonrpc.InternalError, msg)
	}
}

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
	result.CallerAddress = utils.Ptr(info.Call.CallerAddress)
	result.ClassHash = info.Call.ClassHash
	result.EntryPointType = EntryPointType(info.Call.EntryPointType)
	result.CallType = CallType(info.Call.CallType)
	result.Result = info.Execution.Retdata
	result.Calls = utils.Map(info.InnerCalls, func(innerInfo vm.CallInfo) FunctionInvocation {
		return adaptCallInfo(&innerInfo)
	})
	result.Events = utils.Map(info.Execution.Events, func(event vm.OrderedEvent) EventContent {
		e := event.Event
		return EventContent{
			Keys: e.Keys,
			Data: e.Data,
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
