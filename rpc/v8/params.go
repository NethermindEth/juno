package rpcv8

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
)

// Param structs for the JSON-RPC method bindings in MethodsV0_8.
// See rpc/v10/params.go for design notes. v8 has by-value receivers
// on several handlers (BlockID, felt.Felt) so the param fields use
// value types where the underlying handler does.

type BlockIDParams struct {
	BlockID *BlockID `json:"block_id"`
}

type BlockIDValueParams struct {
	BlockID BlockID `json:"block_id"`
}

type TxHashParams struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

type TxHashValueParams struct {
	TransactionHash felt.Felt `json:"transaction_hash"`
}

type ClassHashParams struct {
	ClassHash *felt.Felt `json:"class_hash"`
}

type TxByBlockIDAndIndexParams struct {
	BlockID *BlockID `json:"block_id"`
	Index   int      `json:"index"`
}

type NonceParams struct {
	BlockID         BlockID   `json:"block_id"`
	ContractAddress felt.Felt `json:"contract_address"`
}

type StorageAtParams struct {
	ContractAddress *felt.Felt `json:"contract_address"`
	Key             *felt.Felt `json:"key"`
	BlockID         *BlockID   `json:"block_id"`
}

type ClassParams struct {
	BlockID   BlockID   `json:"block_id"`
	ClassHash felt.Felt `json:"class_hash"`
}

type ClassAtParams struct {
	BlockID         BlockID   `json:"block_id"`
	ContractAddress felt.Felt `json:"contract_address"`
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
	Filter EventsArg `json:"filter"`
}

type CallParams struct {
	Request *FunctionCall `json:"request"`
	BlockID *BlockID      `json:"block_id"`
}

type EstimateFeeParams struct {
	Request         BroadcastedTransactionInputs `json:"request"`
	SimulationFlags []SimulationFlag             `json:"simulation_flags"`
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

type SubscribeEventsParams struct {
	FromAddress *felt.Address        `json:"from_address,omitempty"`
	Keys        [][]felt.Felt        `json:"keys,omitempty"`
	BlockID     *SubscriptionBlockID `json:"block_id,omitempty"`
}

type SubscribeNewHeadsParams struct {
	BlockID *SubscriptionBlockID `json:"block_id,omitempty"`
}

type SubscribePendingTxsParams struct {
	TransactionDetails *bool       `json:"transaction_details,omitempty"`
	SenderAddress      []felt.Felt `json:"sender_address,omitempty"`
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
