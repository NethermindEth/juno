package rpcv9

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

type EventArgs struct {
	EventFilter
	ResultPageRequest
}

type ResultPageRequest struct {
	ContinuationToken string `json:"continuation_token"`
	ChunkSize         uint64 `json:"chunk_size" validate:"min=1"`
}

type EventFilter struct {
	FromBlock *BlockID      `json:"from_block"`
	ToBlock   *BlockID      `json:"to_block"`
	Address   *felt.Address `json:"address"`
	Keys      [][]felt.Felt `json:"keys"`
}

type EventsChunk struct {
	Events            []EmittedEvent `json:"events"`
	ContinuationToken string         `json:"continuation_token,omitempty"`
}

type EmittedEvent struct {
	*Event
	BlockNumber     *uint64    `json:"block_number,omitempty"`
	BlockHash       *felt.Felt `json:"block_hash,omitempty"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

func setEventFilterRange(filter blockchain.EventFilterer, from, to *BlockID, latestHeight uint64) error {
	set := func(filterRange blockchain.EventFilterRange, blockID *BlockID) error {
		if blockID == nil {
			return nil
		}

		switch blockID.Type() {
		case preConfirmed:
			return filter.SetRangeEndBlockByNumber(filterRange, ^uint64(0))
		case latest:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight)
		case hash:
			return filter.SetRangeEndBlockByHash(filterRange, blockID.Hash())
		case number:
			if filterRange == blockchain.EventFilterTo {
				return filter.SetRangeEndBlockByNumber(filterRange, min(blockID.Number(), latestHeight))
			}
			return filter.SetRangeEndBlockByNumber(filterRange, blockID.Number())
		case l1Accepted:
			return filter.SetRangeEndBlockToL1Head(filterRange)
		default:
			panic("Unknown block id type")
		}
	}

	if err := set(blockchain.EventFilterFrom, from); err != nil {
		return err
	}
	return set(blockchain.EventFilterTo, to)
}

/****************************************************
		Events Handlers
*****************************************************/

// Events gets the events matching a filter
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L813
func (h *Handler) Events(args EventArgs) (EventsChunk, *jsonrpc.Error) {
	if args.ChunkSize > rpccore.MaxEventChunkSize {
		return EventsChunk{}, rpccore.ErrPageSizeTooBig
	} else {
		lenKeys := len(args.Keys)
		for _, keys := range args.Keys {
			lenKeys += len(keys)
		}
		if lenKeys > rpccore.MaxEventFilterKeys {
			return EventsChunk{}, rpccore.ErrTooManyKeysInFilter
		}
	}

	height, err := h.bcReader.Height()
	if err != nil {
		return EventsChunk{}, rpccore.ErrInternal
	}

	var addresses []felt.Address
	if args.EventFilter.Address != nil {
		addresses = []felt.Address{*args.EventFilter.Address}
	}
	filter, err := h.bcReader.EventFilter(
		addresses,
		args.EventFilter.Keys,
		h.syncReader.PreConfirmed,
	)
	if err != nil {
		return EventsChunk{}, rpccore.ErrInternal
	}
	filter = filter.WithLimit(h.filterLimit)
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

	var cToken *blockchain.ContinuationToken
	if args.ContinuationToken != "" {
		cToken = new(blockchain.ContinuationToken)
		if err = cToken.FromString(args.ContinuationToken); err != nil {
			return EventsChunk{}, rpccore.ErrInvalidContinuationToken
		}
	}

	if err = setEventFilterRange(
		filter,
		args.EventFilter.FromBlock,
		args.EventFilter.ToBlock,
		height,
	); err != nil {
		return EventsChunk{}, rpccore.ErrBlockNotFound
	}

	filteredEvents, cTokenValue, err := filter.Events(cToken, args.ChunkSize)
	if err != nil {
		return EventsChunk{}, rpccore.ErrInternal
	}

	emittedEvents := make([]EmittedEvent, len(filteredEvents))
	for i, fEvent := range filteredEvents {
		emittedEvents[i] = EmittedEvent{
			BlockNumber:     fEvent.BlockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &Event{
				From: fEvent.From,
				Keys: fEvent.Keys,
				Data: fEvent.Data,
			},
		}
	}

	cTokenStr := ""
	if !cTokenValue.IsEmpty() {
		cTokenStr = cTokenValue.String()
	}
	return EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}
