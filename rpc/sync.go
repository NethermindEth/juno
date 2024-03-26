package rpc

import (
	"encoding/json"
	"github.com/NethermindEth/juno/jsonrpc"

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

	type alias Sync
	return json.Marshal(alias(s))
}

/****************************************************
		Syncing Handlers
*****************************************************/

// Syncing returns the syncing status of the node.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L569
func (h *Handler) Syncing() (*Sync, *jsonrpc.Error) {
	defaultSyncState := &Sync{Syncing: new(bool)}

	startingBlockNumber, err := h.syncReader.StartingBlockNumber()
	if err != nil {
		return defaultSyncState, nil
	}
	startingBlockHeader, err := h.bcReader.BlockHeaderByNumber(startingBlockNumber)
	if err != nil {
		return defaultSyncState, nil
	}
	head, err := h.bcReader.HeadsHeader()
	if err != nil {
		return defaultSyncState, nil
	}
	highestBlockHeader := h.syncReader.HighestBlockHeader()
	if highestBlockHeader == nil {
		return defaultSyncState, nil
	}
	if highestBlockHeader.Number <= head.Number {
		return defaultSyncState, nil
	}

	return &Sync{
		StartingBlockHash:   startingBlockHeader.Hash,
		StartingBlockNumber: &startingBlockHeader.Number,
		CurrentBlockHash:    head.Hash,
		CurrentBlockNumber:  &head.Number,
		HighestBlockHash:    highestBlockHeader.Hash,
		HighestBlockNumber:  &highestBlockHeader.Number,
	}, nil
}
