package rpccore

// PreConfirmedCursor suppresses duplicate live pre_confirmed notifications (events,
// receipts, transactions). The tip is re-published in full on every delta, but within one
// round (block number + identifier) it only grows by appending transactions, so the number
// of transactions already processed is all that is needed. A new block number or round
// identifier (same-height replacement) starts again from the first transaction. Committed
// blocks bypass it.
//
// Not safe for concurrent use: each subscription owns one instance, touched only from its
// single dispatch goroutine. Don't share an instance or add a goroutine without restoring
// synchronisation.
type PreConfirmedCursor struct {
	blockNum   uint64
	identifier string
	processed  int
}

// NewPreConfirmedCursor creates a pre_confirmed tip cursor.
func NewPreConfirmedCursor() *PreConfirmedCursor {
	return &PreConfirmedCursor{}
}

// Advance returns the index of the first of the round's txCount transactions not processed
// yet, and records all txCount as processed. Advancing to a different block number or round
// identifier forgets the previous round first.
func (c *PreConfirmedCursor) Advance(blockNum uint64, identifier string, txCount int) int {
	if blockNum != c.blockNum || identifier != c.identifier {
		*c = PreConfirmedCursor{blockNum: blockNum, identifier: identifier}
	}
	if txCount <= c.processed {
		return txCount // stale or unchanged snapshot: empty range, nothing to send
	}
	from := c.processed
	c.processed = txCount
	return from
}

// Clear forgets the current round.
func (c *PreConfirmedCursor) Clear() {
	*c = PreConfirmedCursor{}
}
