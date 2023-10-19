package starknet

import "github.com/NethermindEth/juno/core/felt"

type BlockTrace struct {
	Traces []TransactionTrace `json:"traces"`
}

type TransactionTrace struct {
	TransactionHash       felt.Felt           `json:"transaction_hash"`
	RevertError           string              `json:"revert_error"`
	ValidateInvocation    *FunctionInvocation `json:"validate_invocation"`
	FunctionInvocation    *FunctionInvocation `json:"function_invocation"`
	FeeTransferInvocation *FunctionInvocation `json:"fee_transfer_invocation"`
	Signature             []felt.Felt         `json:"signature"`
}

type FunctionInvocation struct {
	CallerAddress      felt.Felt              `json:"caller_address"`
	ContractAddress    felt.Felt              `json:"contract_address"`
	Calldata           []felt.Felt            `json:"calldata"`
	CallType           string                 `json:"call_type"`
	ClassHash          *felt.Felt             `json:"class_hash"`
	Selector           *felt.Felt             `json:"selector"`
	EntryPointType     string                 `json:"entry_point_type"`
	Result             []felt.Felt            `json:"result"`
	ExecutionResources ExecutionResources     `json:"execution_resources"`
	InternalCalls      []FunctionInvocation   `json:"internal_calls"`
	Events             []OrderedEvent         `json:"events"`
	Messages           []OrderedL2toL1Message `json:"messages"`
}

type OrderedEvent struct {
	Order uint64      `json:"order"`
	Keys  []felt.Felt `json:"keys"`
	Data  []felt.Felt `json:"data"`
}

type OrderedL2toL1Message struct {
	Order   uint64      `json:"order"`
	ToAddr  string      `json:"to_address"`
	Payload []felt.Felt `json:"payload"`
}
