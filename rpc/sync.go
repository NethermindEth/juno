package rpc

import "github.com/NethermindEth/juno/core/felt"

type SyncState struct {
	False  bool
	Status SyncStatus
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L852
type SyncStatus struct {
	StartingBlockHash   *felt.Felt `json:"starting_block_hash"`
	StartingBlockNumber uint64     `json:"starting_block_number"`
	CurrentBlockHash    *felt.Felt `json:"current_block_hash"`
	CurrentBlockNumber  uint64     `json:"current_block_number"`
	HighestBlockHash    *felt.Felt `json:"highest_block_hash"`
	HighestBlockNumber  uint64     `json:"highest_block_number"`
}
