package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type NumAsHex uint64

func (n NumAsHex) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"0x%x"`, n)), nil
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L852
type Sync struct {
	Syncing             *bool      `json:"-"`
	StartingBlockHash   *felt.Felt `json:"starting_block_hash,omitempty"`
	StartingBlockNumber NumAsHex   `json:"starting_block_number,omitempty"`
	CurrentBlockHash    *felt.Felt `json:"current_block_hash,omitempty"`
	CurrentBlockNumber  NumAsHex   `json:"current_block_number,omitempty"`
	HighestBlockHash    *felt.Felt `json:"highest_block_hash,omitempty"`
	HighestBlockNumber  NumAsHex   `json:"highest_block_number,omitempty"`
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
