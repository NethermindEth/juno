package rpcv9

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
)

type EventArgs struct {
	EventFilter
	rpcv6.ResultPageRequest
}

type EventFilter struct {
	FromBlock *BlockID      `json:"from_block"`
	ToBlock   *BlockID      `json:"to_block"`
	Address   *felt.Felt    `json:"address"`
	Keys      [][]felt.Felt `json:"keys"`
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
func (h *Handler) Events(args EventArgs) (rpcv6.EventsChunk, *jsonrpc.Error) {
	if args.ChunkSize > rpccore.MaxEventChunkSize {
		return rpcv6.EventsChunk{}, rpccore.ErrPageSizeTooBig
	} else {
		lenKeys := len(args.Keys)
		for _, keys := range args.Keys {
			lenKeys += len(keys)
		}
		if lenKeys > rpccore.MaxEventFilterKeys {
			return rpcv6.EventsChunk{}, rpccore.ErrTooManyKeysInFilter
		}
	}

	height, err := h.bcReader.Height()
	if err != nil {
		return rpcv6.EventsChunk{}, rpccore.ErrInternal
	}

	filter, err := h.bcReader.EventFilter(
		[]*felt.Felt{args.EventFilter.Address},
		args.EventFilter.Keys,
		h.PendingData,
	)
	if err != nil {
		return rpcv6.EventsChunk{}, rpccore.ErrInternal
	}
	filter = filter.WithLimit(h.filterLimit)
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

	var cToken *blockchain.ContinuationToken
	if args.ContinuationToken != "" {
		cToken = new(blockchain.ContinuationToken)
		if err = cToken.FromString(args.ContinuationToken); err != nil {
			return rpcv6.EventsChunk{}, rpccore.ErrInvalidContinuationToken
		}
	}

	if err = setEventFilterRange(
		filter,
		args.EventFilter.FromBlock,
		args.EventFilter.ToBlock,
		height,
	); err != nil {
		return rpcv6.EventsChunk{}, rpccore.ErrBlockNotFound
	}

	filteredEvents, cTokenValue, err := filter.Events(cToken, args.ChunkSize)
	if err != nil {
		return rpcv6.EventsChunk{}, rpccore.ErrInternal
	}

	emittedEvents := make([]rpcv6.EmittedEvent, len(filteredEvents))
	for i, fEvent := range filteredEvents {
		emittedEvents[i] = rpcv6.EmittedEvent{
			BlockNumber:     fEvent.BlockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &rpcv6.Event{
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
	return rpcv6.EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}
