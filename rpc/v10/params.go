package rpcv10

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
)

// Param structs for the JSON-RPC method bindings in MethodsV0_10
// (rpc/handlers.go). One struct per method shape. Field names use
// json tags whose values match the JSON-RPC parameter names; the
// `omitempty` suffix marks a parameter as optional.
//
// Many handlers share a shape, so structs are reused where applicable.
// The dispatch closure in handlers.go translates struct fields back
// into positional handler arguments.

// ---------- shared shapes ----------

type BlockIDParams struct {
	BlockID *BlockID `json:"block_id"`
}

type BlockIDFlagsParams struct {
	BlockID       *BlockID      `json:"block_id"`
	ResponseFlags ResponseFlags `json:"response_flags,omitzero"`
}

type TxHashParams struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

type TxHashFlagsParams struct {
	TransactionHash *felt.Felt    `json:"transaction_hash"`
	ResponseFlags   ResponseFlags `json:"response_flags,omitzero"`
}

type ClassHashParams struct {
	ClassHash *felt.Felt `json:"class_hash"`
}

// ---------- per-method shapes ----------

type TxByBlockIDAndIndexParams struct {
	BlockID       *BlockID      `json:"block_id"`
	Index         int           `json:"index"`
	ResponseFlags ResponseFlags `json:"response_flags,omitzero"`
}

type StateUpdateParams struct {
	BlockID           *BlockID    `json:"block_id"`
	ContractAddresses AddressList `json:"contract_addresses,omitempty"`
}

type NonceParams struct {
	BlockID         *BlockID   `json:"block_id"`
	ContractAddress *felt.Felt `json:"contract_address"`
}

type StorageAtParams struct {
	ContractAddress *felt.Address          `json:"contract_address"`
	Key             *felt.Felt             `json:"key"`
	BlockID         *BlockID               `json:"block_id"`
	Flags           StorageAtResponseFlags `json:"response_flags,omitzero"`
}

type ClassParams struct {
	BlockID   *BlockID   `json:"block_id"`
	ClassHash *felt.Felt `json:"class_hash"`
}

type ClassAtParams struct {
	BlockID         *BlockID   `json:"block_id"`
	ContractAddress *felt.Felt `json:"contract_address"`
}

type AddInvokeTxParams struct {
	InvokeTransaction *BroadcastedTransaction `json:"invoke_transaction"`
}

type AddDeployAccountTxParams struct {
	DeployAccountTransaction *BroadcastedTransaction `json:"deploy_account_transaction"`
}

type AddDeclareTxParams struct {
	DeclareTransaction *BroadcastedTransaction `json:"declare_transaction"`
}

type EventsParams struct {
	Filter *EventArgs `json:"filter"`
}

type CallParams struct {
	Request *FunctionCall `json:"request"`
	BlockID *BlockID      `json:"block_id"`
}

type EstimateFeeParams struct {
	Request         BroadcastedTransactionInputs `json:"request"`
	SimulationFlags []EstimateFlag               `json:"simulation_flags"`
	BlockID         *BlockID                     `json:"block_id"`
}

type EstimateMessageFeeParams struct {
	Message *MsgFromL1 `json:"message"`
	BlockID *BlockID   `json:"block_id"`
}

type SimulateTransactionsParams struct {
	BlockID         *BlockID                     `json:"block_id"`
	Transactions    BroadcastedTransactionInputs `json:"transactions"`
	SimulationFlags []SimulationFlag             `json:"simulation_flags"`
}

type TraceBlockTransactionsParams struct {
	BlockID    *BlockID    `json:"block_id"`
	TraceFlags []TraceFlag `json:"trace_flags,omitempty"`
}

type SubscribeEventsParams struct {
	FromAddress    AddressList                 `json:"from_address,omitempty"`
	Keys           [][]felt.Felt               `json:"keys,omitempty"`
	BlockID        *SubscriptionBlockID        `json:"block_id,omitempty"`
	FinalityStatus *TxnFinalityStatusWithoutL1 `json:"finality_status,omitempty"`
}

type SubscribeNewTransactionReceiptsParams struct {
	FinalityStatus []TxnFinalityStatusWithoutL1 `json:"finality_status,omitempty"`
	SenderAddress  []felt.Address               `json:"sender_address,omitempty"`
}

type SubscribeNewHeadsParams struct {
	BlockID *SubscriptionBlockID `json:"block_id,omitempty"`
}

type SubscribeNewTransactionsParams struct {
	FinalityStatus []TxnStatusWithoutL1 `json:"finality_status,omitempty"`
	SenderAddress  []felt.Address       `json:"sender_address,omitempty"`
	Tags           SubscriptionTags     `json:"tags,omitzero"`
}

type UnsubscribeParams struct {
	SubscriptionID string `json:"subscription_id"`
}

type GetMessageStatusParams struct {
	TransactionHash *common.Hash `json:"transaction_hash"`
}

type StorageProofParams struct {
	BlockID              *BlockID      `json:"block_id"`
	ClassHashes          []felt.Felt   `json:"class_hashes,omitempty"`
	ContractAddresses    []felt.Felt   `json:"contract_addresses,omitempty"`
	ContractsStorageKeys []StorageKeys `json:"contracts_storage_keys,omitempty"`
}
