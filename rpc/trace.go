package rpc

import "github.com/NethermindEth/juno/core/felt"

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
