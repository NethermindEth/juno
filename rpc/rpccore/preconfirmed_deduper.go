package rpccore

// PreConfirmedDeduper suppresses duplicate live pre_confirmed notifications (events,
// receipts, transactions). The tip is re-published in full on every delta, so the keys
// already sent this round are all that is needed; a new block number or round identifier
// (same-height replacement) discards them and re-emits. Committed blocks bypass it.
//
// Not safe for concurrent use: each subscription owns one instance, touched only from its
// single dispatch goroutine. Don't share an instance or add a goroutine without restoring
// synchronisation.
type PreConfirmedDeduper[K comparable] struct {
	blockNum   uint64
	identifier string
	seen       map[K]struct{}
}

// NewPreConfirmedDeduper creates a pre_confirmed tip deduper.
func NewPreConfirmedDeduper[K comparable]() *PreConfirmedDeduper[K] {
	return &PreConfirmedDeduper[K]{seen: make(map[K]struct{})}
}

// MarkSent records key for the given pre_confirmed round (blockNum + identifier) and
// reports whether it was newly added — i.e. whether it should be sent. Advancing to a
// different block number or round identifier discards the previous round's keys first.
func (c *PreConfirmedDeduper[K]) MarkSent(blockNum uint64, identifier string, key *K) bool {
	if blockNum != c.blockNum || identifier != c.identifier {
		clear(c.seen)
		c.blockNum = blockNum
		c.identifier = identifier
	}
	if _, ok := c.seen[*key]; ok {
		return false
	}
	c.seen[*key] = struct{}{}
	return true
}

// Clear removes all cached data.
func (c *PreConfirmedDeduper[K]) Clear() {
	clear(c.seen)
	c.blockNum = 0
	c.identifier = ""
}
