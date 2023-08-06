package rpc

import (
	"encoding/json"

	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L852
type Sync struct {
	Syncing             *bool      `json:"-"`
	StartingBlockHash   *felt.Felt `json:"starting_block_hash,omitempty"`
	StartingBlockNumber *uint64    `json:"starting_block_num,omitempty"`
	CurrentBlockHash    *felt.Felt `json:"current_block_hash,omitempty"`
	CurrentBlockNumber  *uint64    `json:"current_block_num,omitempty"`
	HighestBlockHash    *felt.Felt `json:"highest_block_hash,omitempty"`
	HighestBlockNumber  *uint64    `json:"highest_block_num,omitempty"`
}

func (s Sync) MarshalJSON() ([]byte, error) {
	if s.Syncing != nil && !*s.Syncing {
		return json.Marshal(false)
	}
	type Alias Sync
	return json.Marshal(&struct {
		Alias
	}{
		Alias: Alias(s),
	})
}
