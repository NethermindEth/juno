package rpc

import "github.com/NethermindEth/juno/core/felt"

type EventsArg struct {
	EventFilter
	ResultPageRequest
}

type EventFilter struct {
	FromBlock *BlockID     `json:"from_block"`
	ToBlock   *BlockID     `json:"to_block"`
	Address   *felt.Felt   `json:"address"`
	Keys      []*felt.Felt `json:"keys"`
}

type ResultPageRequest struct {
	ContinuationToken string `json:"continuation_token"`
	ChunkSize         uint64 `json:"chunk_size" validate:"min=1"`
}

type EventsChunk struct {
	Events            []*EmittedEvent `json:"events"`
	ContinuationToken string          `json:"continuation_token,omitempty"`
}

type EmittedEvent struct {
	*Event
	BlockNumber     uint64     `json:"block_number"`
	BlockHash       *felt.Felt `json:"block_hash"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}
