package rpc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type EventsArg struct {
	*EventFilter
	*ResultPageRequest
}

func (e *EventsArg) UnmarshalJSON(data []byte) error {
	filter := new(EventFilter)
	if err := json.Unmarshal(data, filter); err != nil {
		return err
	} else if filter.FromBlock == nil || filter.ToBlock == nil || filter.Address == nil || filter.Keys == nil {
		return errors.New("event filter is missing a required field")
	}

	resultPageRequest := new(ResultPageRequest)
	if err := json.Unmarshal(data, resultPageRequest); err != nil {
		return err
	} else if resultPageRequest.ChunkSize < 1 {
		return fmt.Errorf("invalid chunk size (must be greater than 1): %d", resultPageRequest.ChunkSize)
	}

	*e = EventsArg{
		EventFilter:       filter,
		ResultPageRequest: resultPageRequest,
	}
	return nil
}

type EventFilter struct {
	FromBlock *BlockID     `json:"from_block"`
	ToBlock   *BlockID     `json:"to_block"`
	Address   *felt.Felt   `json:"address"`
	Keys      []*felt.Felt `json:"keys"`
}

type ResultPageRequest struct {
	ContinuationToken string `json:"continuation_token"`
	ChunkSize         uint64 `json:"chunk_size"`
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
