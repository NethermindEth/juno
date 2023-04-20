package rpc

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type NumAsHex uint64

func (n NumAsHex) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"0x%x"`, n)), nil
}

type SyncState struct {
	False  *bool       `json:"false,omitempty"`
	Status *SyncStatus `json:"sync_status,omitempty"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L852
type SyncStatus struct {
	StartingBlockHash   *felt.Felt `json:"starting_block_hash"`
	StartingBlockNumber NumAsHex   `json:"starting_block_number"`
	CurrentBlockHash    *felt.Felt `json:"current_block_hash"`
	CurrentBlockNumber  NumAsHex   `json:"current_block_number"`
	HighestBlockHash    *felt.Felt `json:"highest_block_hash"`
	HighestBlockNumber  NumAsHex   `json:"highest_block_number"`
}
