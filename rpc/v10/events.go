package rpcv10

import (
	"encoding/json"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
)

type EventArgs struct {
	EventFilter
	rpcv6.ResultPageRequest
}

type EventFilter struct {
	FromBlock *rpcv9.BlockID `json:"from_block"`
	ToBlock   *rpcv9.BlockID `json:"to_block"`
	Address   addressOrList  `json:"address"`
	Keys      [][]felt.Felt  `json:"keys"`
}

type addressOrList []felt.Felt

func (a *addressOrList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "[]" {
		*a = make([]felt.Felt, 0)
		return nil
	}

	var single felt.Felt
	if err := json.Unmarshal(data, &single); err == nil {
		*a = []felt.Felt{single}
		return nil
	}

	var list []*felt.Felt
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		*a = nil
		return nil
	}
	seen := make(map[felt.Felt]struct{}, len(list))
	unique := make([]felt.Felt, 0, len(list))
	for _, addr := range list {
		if addr != nil {
			if _, exists := seen[*addr]; !exists {
				seen[*addr] = struct{}{}
				unique = append(unique, *addr)
			}
		}
	}
	*a = unique
	return nil
}

type EmittedEvent struct {
	*rpcv6.Event
	BlockNumber      *uint64    `json:"block_number,omitempty"`
	BlockHash        *felt.Felt `json:"block_hash,omitempty"`
	TransactionHash  *felt.Felt `json:"transaction_hash"`
	TransactionIndex uint       `json:"transaction_index"`
	EventIndex       uint       `json:"event_index"`
}

type EventsChunk struct {
	Events            []EmittedEvent `json:"events"`
	ContinuationToken string         `json:"continuation_token,omitempty"`
}

func setEventFilterRange(
	filter blockchain.EventFilterer,
	from,
	to *rpcv9.BlockID,
	latestHeight uint64,
) error {
	set := func(filterRange blockchain.EventFilterRange, blockID *rpcv9.BlockID) error {
		if blockID == nil {
			return nil
		}

		switch {
		case blockID.IsPreConfirmed():
			return filter.SetRangeEndBlockByNumber(filterRange, ^uint64(0))
		case blockID.IsLatest():
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight)
		case blockID.IsHash():
			return filter.SetRangeEndBlockByHash(filterRange, blockID.Hash())
		case blockID.IsNumber():
			if filterRange == blockchain.EventFilterTo {
				return filter.SetRangeEndBlockByNumber(filterRange, min(blockID.Number(), latestHeight))
			}
			return filter.SetRangeEndBlockByNumber(filterRange, blockID.Number())
		case blockID.IsL1Accepted():
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
// https://github.com/starkware-libs/starknet-specs/blob/785257f27cdc4ea0ca3b62a21b0f7bf51000f9b1/api/starknet_api_openrpc.json#L810 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
func (h *Handler) Events(args *EventArgs) (EventsChunk, *jsonrpc.Error) {
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

	filter, err := h.bcReader.EventFilter(
		args.EventFilter.Address,
		args.EventFilter.Keys,
		h.PendingData,
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
			BlockNumber:      fEvent.BlockNumber,
			BlockHash:        fEvent.BlockHash,
			TransactionHash:  fEvent.TransactionHash,
			TransactionIndex: fEvent.TransactionIndex,
			EventIndex:       fEvent.EventIndex,
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
	return EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}
