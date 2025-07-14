package rpcv8

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
)

type SubscriptionID string

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

type EmittedEvent struct {
	*Event
	BlockNumber     *uint64    `json:"block_number,omitempty"`
	BlockHash       *felt.Felt `json:"block_hash,omitempty"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

func (h *Handler) unsubscribe(sub *subscription, id string) {
	sub.cancel()
	h.subscriptions.Delete(id)
}

func setEventFilterRange(filter blockchain.EventFilterer, from, to *BlockID, latestHeight uint64) error {
	set := func(filterRange blockchain.EventFilterRange, blockID *BlockID) error {
		if blockID == nil {
			return nil
		}

		switch blockID.Type() {
		case pending:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight+1)
		case latest:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight)
		case hash:
			return filter.SetRangeEndBlockByHash(filterRange, blockID.Hash())
		case number:
			if filterRange == blockchain.EventFilterTo {
				return filter.SetRangeEndBlockByNumber(filterRange, min(blockID.Number(), latestHeight))
			}
			return filter.SetRangeEndBlockByNumber(filterRange, blockID.Number())
		default:
			panic("Unknown block id type")
		}
	}

	if err := set(blockchain.EventFilterFrom, from); err != nil {
		return err
	}
	return set(blockchain.EventFilterTo, to)
}
