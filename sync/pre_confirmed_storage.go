package sync

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/starknet"
)

// PreConfirmedStorage owns the atomically-stored pre_confirmed block and the
// rules for evolving it under a wire-side update.
type PreConfirmedStorage struct {
	inner atomic.Pointer[pending.PreConfirmed]
}

func NewPreConfirmedStorage() *PreConfirmedStorage {
	return &PreConfirmedStorage{}
}

// ReadUnsafe returns the currently stored pre_confirmed pointer without any
// validation against the canonical head. The result may be nil, stale, or
// inconsistent with the chain — callers that need a consumer-safe view should
// use ReadPreConfirmedForHead instead.
func (s *PreConfirmedStorage) ReadUnsafe() *pending.PreConfirmed {
	return s.inner.Load()
}

// ReadPreConfirmedForHead returns the stored pre_confirmed if it is a valid
// successor to head (head+1, or head+2 with a valid PreLatest in between).
// Returns nil if nothing is stored or the stored pre_confirmed is stale.
//
// head MUST be the canonical chain head as returned by
// blockchain.HeadsHeader(), or nil when the chain is empty (genesis).
func (s *PreConfirmedStorage) ReadPreConfirmedForHead(head *core.Header) *pending.PreConfirmed {
	p := s.inner.Load()
	if p == nil || !p.Validate(head) {
		return nil
	}
	// Special handling: if the pre-confirmed contains a 'pre-latest' block attachment
	// that is now outdated (head moved on), return a copy with the pre-latest attachment discarded.
	if head != nil && p.Block.Number == head.Number+1 && p.PreLatest != nil {
		return p.Copy().WithPreLatest(nil)
	}
	return p
}

// ApplyUpdate atomically evolves the stored pre_confirmed from a wire-side
// update, attaching the given pre_latest, under the preserve-if-richer rule.
// A [starknet.PreConfirmedFull] update bootstraps the store when nothing is
// yet stored; [starknet.PreConfirmedDelta] and [starknet.PreConfirmedNoChange]
// are no-ops in that case (Delta needs a baseline to merge into).
// Returns the resulting pre_confirmed if the store was replaced; nil otherwise.
//
// head MUST be the canonical chain head from blockchain.HeadsHeader(), or
// nil at genesis. It is forwarded to StorePreConfirmedForHead for the
// validation gate — see that method for the contract.
func (s *PreConfirmedStorage) ApplyUpdate(
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	head *core.Header,
	preLatest *pending.PreLatest,
) (*pending.PreConfirmed, error) {
	current := s.inner.Load()

	var next pending.PreConfirmed
	var err error
	switch u := update.(type) {
	case starknet.PreConfirmedNoChange:
		return nil, nil

	case starknet.PreConfirmedFull:
		next, err = sn2core.AdaptPreConfirmedBlock(&u, blockNumber)
		if err != nil {
			return nil, err
		}

	case starknet.PreConfirmedDelta:
		if current == nil {
			return nil, nil
		}
		next, err = sn2core.AdaptPreConfirmedWithDelta(current, &u)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown PreConfirmedUpdate variant %T", update)
	}
	next.WithPreLatest(preLatest)

	changed, err := s.StorePreConfirmedForHead(&next, head)
	if err != nil || !changed {
		return nil, err
	}
	return &next, nil
}

// StorePreConfirmedForHead atomically stores a fully-constructed pre_confirmed.
// The protocol version must be supported and the block must validate against
// head. The store-vs-preserve decision then follows the rules below; returns
// true if the stored pointer was replaced.
//
// Replacement happens when:
//   - the incoming block is at a higher number than the existing one, or
//   - same number but a different BlockIdentifier (new round) — replaces even
//     if the new block has fewer txs, or
//   - same number and identifier but the incoming block is strictly richer
//     (more transactions).
//
// Otherwise the existing block is preserved; if the incoming carries a fresh
// PreLatest attachment we refresh that in-place via CAS without swapping the
// pre_confirmed pointer.
//
// head MUST be the canonical chain head as returned by
// blockchain.HeadsHeader(), or nil when the chain is empty (genesis).
func (s *PreConfirmedStorage) StorePreConfirmedForHead(
	p *pending.PreConfirmed,
	head *core.Header,
) (bool, error) {
	if err := core.CheckBlockVersion(p.GetBlock().ProtocolVersion); err != nil {
		return false, err
	}

	if !p.Validate(head) {
		return false, errors.New("store pre_confirmed not valid for parent")
	}

	existing := s.inner.Load()

	if existing != nil && shouldPreservePreConfirmed(existing, p, head) {
		_ = s.UpdatePreLatestAttachment(p.GetBlock().Number, p.PreLatest)
		return false, nil
	}

	return s.inner.CompareAndSwap(existing, p), nil
}

// UpdatePreLatestAttachment swaps in a new PreLatest attachment for the stored
// pre_confirmed at the given block number. Returns true on a CAS swap.
func (s *PreConfirmedStorage) UpdatePreLatestAttachment(
	blockNumber uint64,
	preLatest *pending.PreLatest,
) bool {
	pc := s.inner.Load()

	if pc == nil || pc.Block == nil || pc.Block.Number != blockNumber {
		return false
	}

	if pc.PreLatest == preLatest {
		return false
	}

	next := pc.Copy()
	next.WithPreLatest(preLatest)

	return s.inner.CompareAndSwap(pc, next)
}

// shouldPreservePreConfirmed reports whether the existing pre_confirmed is valid
// for head and at least as rich as the incoming candidate.
func shouldPreservePreConfirmed(
	existing *pending.PreConfirmed,
	incoming *pending.PreConfirmed,
	head *core.Header,
) bool {
	if existing == nil {
		return false
	}

	if !existing.Validate(head) {
		return false
	}

	existingB := existing.GetBlock()
	incomingB := incoming.GetBlock()

	if incomingB.Number > existingB.Number {
		return false
	}

	if incomingB.Number == existingB.Number {
		if incoming.BlockIdentifier != existing.BlockIdentifier {
			return false
		}
		if incomingB.TransactionCount > existingB.TransactionCount {
			return false
		}
	}

	return true
}
