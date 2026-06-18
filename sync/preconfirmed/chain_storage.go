package preconfirmed

import (
	"errors"
	"fmt"
	"iter"
	"sync/atomic"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/starknet"
)

// ErrBaseTxCountMismatch is returned when a Delta update's baseTxCount hint
// doesn't match the targeted slot's current tx count. Defensive against any
// non-poller writer (or future race) that could drift the slot between the
// wire send and the storage apply.
var ErrBaseTxCountMismatch = errors.New("pre_confirmed base transaction count mismatch")

// node is one entry in the chain's immutable linked list, pointing back
// toward older blocks via parent. Nodes are never mutated in place — every
// storage write produces fresh nodes for the affected slot and everything
// newer than it, so concurrent readers walking a prior snapshot see a stable
// graph. Popped nodes become unreferenced and GC-collectable.
type node struct {
	pc     *pending.PreConfirmed
	parent *node
}

// ChainReader is an immutable snapshot of a contiguous run of pre-confirmed
// blocks, ordered newest-first via parent pointers. Iteration must respect Length
// — head-aligned views (see ChainStorage.SnapshotForHead) may stop
// before the underlying linked list's nil terminator.
type ChainReader struct {
	head   *node
	length int
}

// Length is the number of entries in this chain view.
func (c *ChainReader) Length() int {
	if c == nil {
		return 0
	}
	return c.length
}

// Head returns the most recent pre-confirmed in the view, or nil if empty.
func (c *ChainReader) Head() *pending.PreConfirmed {
	if c == nil || c.length == 0 {
		return nil
	}
	return c.head.pc
}

// NewestFirst yields entries from the most recent down to head+1, bounded by Length.
func (c *ChainReader) NewestFirst() iter.Seq[*pending.PreConfirmed] {
	return func(yield func(*pending.PreConfirmed) bool) {
		if c == nil {
			return
		}
		n := c.head
		for i := 0; i < c.length && n != nil; i++ {
			if !yield(n.pc) {
				return
			}
			n = n.parent
		}
	}
}

// OldestFirst yields entries from head+1 up to the most recent, bounded by Length.
func (c *ChainReader) OldestFirst() iter.Seq[*pending.PreConfirmed] {
	return func(yield func(*pending.PreConfirmed) bool) {
		if c == nil {
			return
		}
		walkOldestFirst(c.head, c.length, yield)
	}
}

// TransactionByHash scans every chain entry's Block.Transactions.
//
// Returns [pending.ErrTransactionNotFound] when missing.
func (c *ChainReader) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	if c == nil || c.length == 0 {
		return nil, pending.ErrTransactionNotFound
	}
	for entry := range c.NewestFirst() {
		for _, tx := range entry.Block.Transactions {
			if tx.Hash().Equal(hash) {
				return tx, nil
			}
		}
	}

	return nil, pending.ErrTransactionNotFound
}

// ReceiptByHash scans every chain entry's Block.Receipts. Returns the receipt
// and the number of the block it lives in. ErrTransactionReceiptNotFound when
// missing.
func (c *ChainReader) ReceiptByHash(
	hash *felt.Felt,
) (*core.TransactionReceipt, uint64, error) {
	if c == nil || c.length == 0 {
		return nil, 0, pending.ErrTransactionReceiptNotFound
	}
	for entry := range c.NewestFirst() {
		for _, receipt := range entry.Block.Receipts {
			if receipt.TransactionHash.Equal(hash) {
				return receipt, entry.Block.Number, nil
			}
		}
	}
	return nil, 0, pending.ErrTransactionReceiptNotFound
}

// PendingStateAt returns the chain's view of state at blockNumber: baseState
// (canonical state immediately below the chain's bottom) layered with every
// chain entry's full StateDiff from the bottom through blockNumber inclusive.
// Returns [pending.ErrPreConfirmedNotFound] if blockNumber falls outside the
// chain.
func (c *ChainReader) PendingStateAt(
	blockNumber uint64,
	baseState core.StateReader,
) (core.StateReader, error) {
	if c == nil || c.length == 0 {
		return nil, pending.ErrPreConfirmedNotFound
	}
	bottom := c.head.pc.Block.Number - uint64(c.length-1)
	if blockNumber < bottom || blockNumber > c.head.pc.Block.Number {
		return nil, pending.ErrPreConfirmedNotFound
	}
	stateDiff := core.EmptyStateDiff()
	for entry := range c.OldestFirst() {
		stateDiff.Merge(entry.StateUpdate.StateDiff)
		if entry.Block.Number == blockNumber {
			break
		}
	}
	return pending.NewState(&stateDiff, nil, baseState, blockNumber), nil
}

// PendingStateBeforeIndexAt returns the chain's view of state immediately
// before transaction `index` at blockNumber: full StateDiffs of every chain
// entry below blockNumber, then the target's TransactionStateDiffs up to (but
// not including) `index`. Returns [pending.ErrPreConfirmedNotFound] if
// blockNumber isn't in the chain, or
// [pending.ErrTransactionIndexOutOfBounds] if `index` exceeds the target's
// transaction count.
func (c *ChainReader) PendingStateBeforeIndexAt(
	blockNumber uint64,
	index uint,
	baseState core.StateReader,
) (core.StateReader, error) {
	if c == nil || c.length == 0 {
		return nil, pending.ErrPreConfirmedNotFound
	}
	bottom := c.head.pc.Block.Number - uint64(c.length-1)
	if blockNumber < bottom || blockNumber > c.head.pc.Block.Number {
		return nil, pending.ErrPreConfirmedNotFound
	}
	stateDiff := core.EmptyStateDiff()
	var target *pending.PreConfirmed
	for entry := range c.OldestFirst() {
		if entry.Block.Number == blockNumber {
			target = entry
			break
		}
		stateDiff.Merge(entry.StateUpdate.StateDiff)
	}
	if target == nil {
		return nil, pending.ErrPreConfirmedNotFound
	}
	if index > uint(len(target.Block.Transactions)) {
		return nil, pending.ErrTransactionIndexOutOfBounds
	}
	for _, txStateDiff := range target.TransactionStateDiffs[:index] {
		stateDiff.Merge(txStateDiff)
	}
	return pending.NewState(&stateDiff, nil, baseState, blockNumber), nil
}

// walkOldestFirst recurses to the bottom of the chain and yields entries on
// the way back up, producing oldest-first iteration order without
// materialising a slice. remaining bounds the depth so head-aligned views
// stop short of the underlying linked list's nil terminator (relevant after
// SnapshotForHead trims an oversized chain). Returns false when the yield
// callback aborts iteration. Depth equals the caller's Length: BlockHashLag for
// SnapshotForHead views, the full stored chain for UnsafeSnapshot.
func walkOldestFirst(
	n *node,
	remaining int,
	yield func(*pending.PreConfirmed) bool,
) bool {
	if n == nil || remaining == 0 {
		return true
	}
	if !walkOldestFirst(n.parent, remaining-1, yield) {
		return false
	}
	return yield(n.pc)
}

// ChainStorage holds an uncapped contiguous run of pre-confirmed blocks above
// the canonical head. The BlockHashLag visibility cap is enforced reader-side
// by SnapshotForHead, not by the storage. Single writer (polling loop) with
// many concurrent readers; reads are lock-free via atomic.Pointer.
type ChainStorage struct {
	inner atomic.Pointer[ChainReader]
}

func NewChainStorage() *ChainStorage {
	return &ChainStorage{}
}

// UnsafeSnapshot returns the live chain as-is — no head alignment, no
// BlockHashLag cap. Use [SnapshotForHead] to trim stale entries below head+1
// and to bound the view at head+BlockHashLag; this is for the poller's
// internal use. Returns nil when storage is empty.
func (s *ChainStorage) UnsafeSnapshot() *ChainReader {
	return s.inner.Load()
}

// SnapshotForHead returns a chain view bounded by the canonical head on both
// ends:
//
//   - Lower bound: conceptual bottom is head.Number+1 (or 0 at genesis with
//     head == nil). If the stored chain extends below head+1 (storage briefly
//     stale because AdvanceTo hasn't run yet against this head advance), the
//     view reports a shorter length, excluding now-committed entries.
//   - Upper bound: the view exposes at most BlockHashLag entries (head+1 .. head+
//     BlockHashLag). Entries above head+BlockHashLag (which storage may transiently
//     hold while the sequencer runs ahead) are excluded; the view's tip walks
//     down from storage's head past them.
//
// Returns the zero-value ChainReader (length 0) if the chain does not cover
// head+1; callers should branch on Length().
func (s *ChainStorage) SnapshotForHead(head *core.Header) ChainReader {
	c := s.inner.Load()
	if c == nil || c.length == 0 {
		return ChainReader{}
	}
	wantBottom := headPlusOne(head)
	storedTip := c.head.pc.Block.Number
	if wantBottom > storedTip {
		return ChainReader{}
	}
	storedBottom := storedTip - uint64(c.length-1)
	if wantBottom < storedBottom {
		return ChainReader{}
	}

	// Upper-bound cap: walk down from storage's head past any entries above
	// head+BlockHashLag so the view's tip is at most head+BlockHashLag.
	upperBound := wantBottom + core.BlockHashLag - 1
	viewHead := c.head
	viewTip := storedTip
	if storedTip > upperBound {
		skip := storedTip - upperBound
		for range skip {
			viewHead = viewHead.parent
		}
		viewTip = upperBound
	}

	want := int(viewTip - wantBottom + 1)
	if viewHead == c.head && want == c.length {
		return *c
	}
	return ChainReader{head: viewHead, length: want}
}

// ApplyUpdate atomically evolves the stored chain from a wire-side update.
// blockNumber is the height targeted by the update; baseTxCount is the
// knownTransactionCount the poller sent (consulted only for the Delta case
// as a defensive race-check against the targeted slot). head MUST be the
// canonical chain head, or nil at genesis. Returns the affected entry, or
// nil when the update was a no-op (NoChange, preserved, rejected at cap, etc.).
//
// On CAS failure the chain changed between Load and CompareAndSwap; we return
// an error instead of retrying.
func (s *ChainStorage) ApplyUpdate(
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	head *core.Header,
) (*pending.PreConfirmed, error) {
	if _, ok := update.(starknet.PreConfirmedNoChange); ok {
		return nil, nil
	}
	current := s.inner.Load()
	newChain, affected, err := computeUpdate(current, update, blockNumber, baseTxCount, head)
	if err != nil {
		return nil, err
	}
	if newChain == nil {
		return nil, nil
	}
	if !s.inner.CompareAndSwap(current, newChain) {
		return nil, errors.New("chain changed between load and store")
	}
	return affected, nil
}

// AdvanceTo realigns the chain to a new canonical head. Three outcomes:
//
//   - wantBottom == bottom: chain already aligned, no-op.
//   - wantBottom > mostRecent (head advanced past everything we stored) OR
//     wantBottom < bottom (head reverted below us — every entry's parent now
//     references a discarded block): drop the whole chain. The next poll
//     bootstraps fresh against the new head.
//   - bottom < wantBottom <= mostRecent: rebuild from the new bottom up so the
//     surviving nodes nil-terminate cleanly and the dropped tail is GC-able.
//
// Pre-pop readers retain their *ChainReader and walk the original (still
// intact) nodes; the new chain references only fresh nodes.
//
// Single-writer: like ApplyUpdate, this assumes the pre_confirmed poller
// goroutine is the only writer.
func (s *ChainStorage) AdvanceTo(head *core.Header) bool {
	current := s.inner.Load()
	if current == nil || current.length == 0 {
		return false
	}
	mostRecent := current.head.pc.Block.Number
	bottom := mostRecent - uint64(current.length-1)
	wantBottom := headPlusOne(head)
	if wantBottom == bottom {
		return false
	}
	if wantBottom < bottom || wantBottom > mostRecent {
		return s.inner.CompareAndSwap(current, nil)
	}
	drop := int(wantBottom - bottom)
	keep := current.length - drop
	newChain := &ChainReader{head: rebuild(current.head, keep), length: keep}
	return s.inner.CompareAndSwap(current, newChain)
}

// rebuild walks `keep` levels down from n via parent pointers, then on the
// way back up builds fresh nodes so the bottom-most has parent==nil. The
// original nodes stay reachable for any concurrent walkers of the old chain
// pointer; once those release, the dropped tail (below the new bottom)
// becomes unreachable and GC-collectable.
func rebuild(n *node, keep int) *node {
	if keep == 0 || n == nil {
		return nil
	}
	child := rebuild(n.parent, keep-1)
	return &node{pc: n.pc, parent: child}
}

// computeUpdate is the pure dispatcher that turns a wire-side update into a
// new chain. Four mutually-exclusive cases:
//
//   - empty chain                       → bootstrapChain (only PreConfirmedBlock accepted)
//   - blockNumber > mostRecent + 1     → gap above tip, reject (no-op return)
//   - blockNumber == mostRecent + 1    → appendMostRecent (extension by one)
//   - blockNumber within [bottom, mostRecent] → replaceSlot (in-chain mutation)
//
// Below-bottom updates and unalignment with head are rejected here too —
// callers must AdvanceTo first to align the chain to a fresh head.
//
// Returns (newChain, affected, err): newChain==nil means "no-op, leave the
// store as-is"; err is reserved for invariant violations (e.g. bottom
// drifted from head, delta baseTxCount mismatch).
func computeUpdate(
	current *ChainReader,
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	head *core.Header,
) (*ChainReader, *pending.PreConfirmed, error) {
	if current == nil || current.length == 0 {
		block, ok := update.(starknet.PreConfirmedBlock)
		if !ok {
			return nil, nil, fmt.Errorf("bootstrap rejected: want PreConfirmedBlock, got %T", update)
		}
		return bootstrapChain(&block, blockNumber, head)
	}

	mostRecent := current.head.pc.Block.Number
	bottom := mostRecent - uint64(current.length-1)

	if !validBottomForHead(bottom, head) {
		return nil, nil, fmt.Errorf("chain bottom %d not aligned with head", bottom)
	}

	// Gap above tip — should never happen under a well-behaved poller, which
	// backfills intermediate heights before applying the latest. Surface as
	// an error so the bug isn't masked as a silent no-op.
	if blockNumber > mostRecent+1 {
		return nil, nil, fmt.Errorf(
			"gap above tip: block %d > mostRecent+1 %d", blockNumber, mostRecent+1,
		)
	}

	// Append as new most recent. Only PreConfirmedBlock makes sense at a
	// brand-new slot; a Delta would have nothing to merge into.
	if blockNumber == mostRecent+1 {
		block, ok := update.(starknet.PreConfirmedBlock)
		if !ok {
			return nil, nil, fmt.Errorf(
				"append rejected at slot %d: want PreConfirmedBlock, got %T", blockNumber, update,
			)
		}
		return extend(current, &block, blockNumber)
	}

	// In-chain update — locate the target slot. blockNumber below bottom means
	// the apply target is at or below the canonical head (bottom == head+1),
	// i.e. already committed; the caller is asking us to write into the past.
	if blockNumber < bottom {
		return nil, nil, fmt.Errorf("apply target %d below bottom %d", blockNumber, bottom)
	}
	return replaceSlot(current, update, blockNumber, baseTxCount)
}

// bootstrapChain handles the first entry case (empty storage). The caller
// (computeUpdate) has already narrowed the update variant to PreConfirmedBlock.
// validBottomForHead is the precondition: blockNumber must equal head.Number+1
// (or 0 at genesis). Returns the new length-1 chain plus the adapted entry.
func bootstrapChain(
	block *starknet.PreConfirmedBlock,
	blockNumber uint64,
	head *core.Header,
) (*ChainReader, *pending.PreConfirmed, error) {
	if !validBottomForHead(blockNumber, head) {
		if head == nil {
			return nil, nil, fmt.Errorf("bootstrap block %d invalid at genesis (want 0)", blockNumber)
		}
		return nil, nil, fmt.Errorf("bootstrap block %d invalid for head %d (want %d)",
			blockNumber, head.Number, head.Number+1)
	}
	next, err := sn2core.AdaptPreConfirmedBlock(block, blockNumber)
	if err != nil {
		return nil, nil, err
	}
	if err := core.CheckBlockVersion(next.Block.ProtocolVersion); err != nil {
		return nil, nil, err
	}
	newNode := &node{pc: &next, parent: nil}
	return &ChainReader{head: newNode, length: 1}, &next, nil
}

// extend grows the chain by one when the incoming block's blockNumber equals
// mostRecent+1.
func extend(
	current *ChainReader,
	block *starknet.PreConfirmedBlock,
	blockNumber uint64,
) (*ChainReader, *pending.PreConfirmed, error) {
	next, err := sn2core.AdaptPreConfirmedBlock(block, blockNumber)
	if err != nil {
		return nil, nil, err
	}
	if err := core.CheckBlockVersion(next.Block.ProtocolVersion); err != nil {
		return nil, nil, err
	}
	newNode := &node{pc: &next, parent: current.head}
	return &ChainReader{head: newNode, length: current.length + 1}, &next, nil
}

// replaceSlot locates the in-chain slot at blockNumber and mutates it,
// dispatching by update variant:
//
//   - PreConfirmedBlock — new round or richer same-round content; shouldPreserveSlot
//     decides whether to keep the existing entry or swap in the incoming one.
//     A non-tip replacement also truncates every node above the replaced slot.
//   - PreConfirmedDeltaUpdate — merges appended txs into the existing slot;
//     baseTxCount must match the slot's current tx count or ErrBaseTxCountMismatch
//     is returned (defensive race-check).
//
// Returns (nil, nil, nil) when shouldPreserveSlot says "keep the existing slot,
// no broadcast needed." Caller must have already validated that blockNumber
// falls within [bottom, mostRecent].
func replaceSlot(
	current *ChainReader,
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
) (*ChainReader, *pending.PreConfirmed, error) {
	depthFromHead := int(current.head.pc.Block.Number - blockNumber)
	target := current.head
	for range depthFromHead {
		target = target.parent
	}
	switch u := update.(type) {
	case starknet.PreConfirmedBlock:
		next, err := sn2core.AdaptPreConfirmedBlock(&u, blockNumber)
		if err != nil {
			return nil, nil, err
		}
		if err := core.CheckBlockVersion(next.Block.ProtocolVersion); err != nil {
			return nil, nil, err
		}
		if shouldPreserveSlot(target.pc, &next) {
			return nil, nil, nil
		}
		newNode := &node{pc: &next, parent: target.parent}
		return &ChainReader{
			head:   newNode,
			length: current.length - depthFromHead,
		}, &next, nil

	case starknet.PreConfirmedDeltaUpdate:
		// Delta updates can only target the chain tip
		if depthFromHead != 0 {
			return nil, nil, fmt.Errorf("delta at non-tip slot %d (depth %d)", blockNumber, depthFromHead)
		}
		if uint64(len(target.pc.Block.Transactions)) != baseTxCount {
			return nil, nil, ErrBaseTxCountMismatch
		}
		next, err := sn2core.AdaptPreConfirmedWithDelta(target.pc, &u)
		if err != nil {
			return nil, nil, err
		}
		newNode := &node{pc: &next, parent: target.parent}
		return &ChainReader{
			head:   newNode,
			length: current.length,
		}, &next, nil
	}
	return nil, nil, fmt.Errorf("unknown PreConfirmedUpdate variant %T", update)
}

// headPlusOne returns the first pre-confirmed slot above the canonical head:
// head.Number+1, or 0 when head is nil (i.e. before the genesis block has
// been ingested).
func headPlusOne(head *core.Header) uint64 {
	if head == nil {
		return 0
	}
	return head.Number + 1
}

// validBottomForHead is the storage's core alignment check: the chain's
// bottom slot must equal head+1, otherwise the stored entries reference
// blocks that are either already committed (head moved past them) or floating
// without a parent. Callers compute bottom independently and pass it in.
func validBottomForHead(bottom uint64, head *core.Header) bool {
	return bottom == headPlusOne(head)
}

// shouldPreserveSlot keeps the existing slot when the incoming pre-confirmed is
// at the same identifier with no extra transactions, or carries the blank
// placeholder identifier. A different real identifier (new round) or a richer
// same-identifier block replaces.
func shouldPreserveSlot(existing, incoming *pending.PreConfirmed) bool {
	if incoming.BlockIdentifier != existing.BlockIdentifier &&
		incoming.BlockIdentifier != feeder.PreConfirmedBlankIdentifier {
		return false
	}
	if incoming.Block.TransactionCount > existing.Block.TransactionCount {
		return false
	}
	return true
}
