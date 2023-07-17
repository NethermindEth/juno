package vm

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
)

type TransactionExecutionInfo struct {
	ValidateCallInfo    *CallInfo `json:"validate_call_info"`
	ExecuteCallInfo     *CallInfo `json:"execute_call_info"`
	FeeTransferCallInfo *CallInfo `json:"fee_transfer_call_info"`
}

type CallInfo struct {
	Call       CallEntryPoint `json:"call"`
	Execution  CallExecution  `json:"execution"`
	InnerCalls []CallInfo     `json:"inner_calls"`
}

type CallEntryPoint struct {
	EntryPointSelector felt.Felt   `json:"entry_point_selector"`
	Calldata           []felt.Felt `json:"calldata"`
	CallerAddress      felt.Felt   `json:"caller_address"`
	ClassHash          *felt.Felt  `json:"class_hash"`
	CodeAddress        *felt.Felt  `json:"code_address"`
	EntryPointType     string      `json:"entry_point_type"`
	CallType           CallType    `json:"call_type"`
}

type CallType string

type CallExecution struct {
	Retdata        []felt.Felt            `json:"retdata"`
	Events         []OrderedEvent         `json:"events"`
	L2ToL1Messages []OrderedL2ToL1Message `json:"l2_to_l1_messages"`
}

type OrderedEvent struct {
	Order int          `json:"order"`
	Event EventContent `json:"event"`
}

type EventContent struct {
	Keys []felt.Felt `json:"keys"`
	Data []felt.Felt `json:"data"`
}

type OrderedL2ToL1Message struct {
	Order   int         `json:"order"`
	Message MessageToL1 `json:"message"`
}

type MessageToL1 struct {
	ToAddress common.Address `json:"to_address"`
	Payload   []felt.Felt    `json:"payload"`
}
