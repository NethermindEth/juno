package rpcv10

import (
	"encoding/json"

	"github.com/NethermindEth/juno/core/felt"
)

type OrderedEvent struct {
	Order uint64       `json:"order"`
	Keys  []*felt.Felt `json:"keys"`
	Data  []*felt.Felt `json:"data"`
}

type OrderedL2toL1Message struct {
	Order   uint64       `json:"order"`
	From    *felt.Felt   `json:"from_address"`
	To      *felt.Felt   `json:"to_address"`
	Payload []*felt.Felt `json:"payload"`
}

type FunctionInvocation struct {
	ContractAddress    felt.Felt                `json:"contract_address"`
	EntryPointSelector *felt.Felt               `json:"entry_point_selector"`
	Calldata           []felt.Felt              `json:"calldata"`
	CallerAddress      felt.Felt                `json:"caller_address"`
	ClassHash          *felt.Felt               `json:"class_hash"`
	EntryPointType     string                   `json:"entry_point_type"`
	CallType           string                   `json:"call_type"`
	Result             []felt.Felt              `json:"result"`
	Calls              []FunctionInvocation     `json:"calls"`
	Events             []OrderedEvent           `json:"events"`
	Messages           []OrderedL2toL1Message   `json:"messages"`
	ExecutionResources *InnerExecutionResources `json:"execution_resources"`
	IsReverted         bool                     `json:"is_reverted"`
}

type ExecuteInvocation struct {
	RevertReason        string `json:"revert_reason"`
	*FunctionInvocation `json:",omitempty"`
}

func (e ExecuteInvocation) MarshalJSON() ([]byte, error) {
	if e.FunctionInvocation != nil {
		return json.Marshal(e.FunctionInvocation)
	}
	type alias ExecuteInvocation
	return json.Marshal(alias(e))
}
