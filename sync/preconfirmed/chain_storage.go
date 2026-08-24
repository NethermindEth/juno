package preconfirmed

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"sync/atomic"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
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
var ErrBaseTxCountMismatch = errors.New("pre-confirmed base transaction count mismatch")

// node is one entry in the chain's immutable linked list, pointing back
// toward older blocks via parent. Nodes are never mutated in place — every
// storage write produces fresh nodes for the affected slot and everything
// newer than it, so concurrent readers walking a prior snapshot see a stable
// graph. Popped nodes become unreferenced and GC-collectable.
type node struct {
	preconfirmed *pending.PreConfirmed
	parent       *node
}

// ChainReader is an immutable snapshot of a contiguous run of pre-confirmed
// blocks, ordered newest-first via parent pointers. Iteration must respect Length
// — head-aligned views (see [ChainStorage.SnapshotForBlock]) may stop
// before the underlying linked list's nil terminator.
type ChainReader struct {
	head   *node
	length int
}

// NewChain builds a [ChainReader] from non-nil pre-confirmed entries given in
// oldest-first order with contiguous block numbers. No args returns the
// zero-value [ChainReader]. If an entry has block number 0 behaviour is undefined.
func NewChain(entries ...*pending.PreConfirmed) (ChainReader, error) {
	if len(entries) == 0 {
		return ChainReader{}, nil
	}
	var head *node
	for index, entry := range entries {
		if entry == nil {
			return ChainReader{}, fmt.Errorf("entry %d is nil", index)
		}

		if index > 0 && entry.Block.Number != entries[index-1].Block.Number+1 {
			return ChainReader{}, fmt.Errorf(
				"non-contiguous block numbers at index %d (%d after %d)",
				index, entry.Block.Number, entries[index-1].Block.Number,
			)
		}
		head = &node{preconfirmed: entry, parent: head}
	}
	return ChainReader{head: head, length: len(entries)}, nil
}

// Length is the number of entries in this chain view.
func (c *ChainReader) Length() int {
	return c.length
}

// Head returns the most recent pre-confirmed in the view, or nil if empty.
func (c *ChainReader) Head() *pending.PreConfirmed {
	if c.length == 0 {
		return nil
	}
	return c.head.preconfirmed
}

// NewestFirst yields entries from the most recent down to head+1, bounded by Length.
func (c *ChainReader) NewestFirst() iter.Seq[*pending.PreConfirmed] {
	return func(yield func(*pending.PreConfirmed) bool) {
		current := c.head
		for count := 0; count < c.length && current != nil; count++ {
			if !yield(current.preconfirmed) {
				return
			}
			current = current.parent
		}
	}
}

// OldestFirst yields entries from head+1 up to the most recent, bounded by Length.
func (c *ChainReader) OldestFirst() iter.Seq[*pending.PreConfirmed] {
	return func(yield func(*pending.PreConfirmed) bool) {
		walkOldestFirst(c.head, c.length, yield)
	}
}

// TransactionByHash scans every chain entry's Block.Transactions.
//
// Returns [pending.ErrTransactionNotFound] when missing.
func (c *ChainReader) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	if c.length == 0 {
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
	// todo(rdr): change to felt.TransactionHash
	hash *felt.Felt,
) (*core.TransactionReceipt, uint64, error) {
	if c.length == 0 {
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

// PreConfirmedStateAt returns the chain's view of state at blockNumber. The chain
// owns base resolution: it opens the canonical state immediately below its
// own oldest slot (derived from tip - length + 1).
//
//	Returns [pending.ErrPreConfirmedNotFound] if blockNumber falls outside the chain.
func (c *ChainReader) PreConfirmedStateAt(
	blockNumber uint64,
	bcReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	if !c.contains(blockNumber) {
		return nil, nil, pending.ErrPreConfirmedNotFound
	}

	base, closer, err := c.baseState(bcReader)
	if err != nil {
		return nil, nil, err
	}

	stateDiff := core.EmptyStateDiff()
	var newClasses map[felt.Felt]core.ClassDefinition
	for entry := range c.OldestFirst() {
		stateDiff.Merge(entry.StateUpdate.StateDiff)
		newClasses = mergeClassesInto(newClasses, entry.NewClasses)
		if entry.Block.Number == blockNumber {
			break
		}
	}
	return pending.NewState(&stateDiff, newClasses, base, blockNumber), closer, nil
}

// PreConfirmedStateBeforeIndexAt returns the chain's view of state immediately
// before transaction `index` at blockNumber. See PreConfirmedStateAt for the base-
// resolution contract; here the chain additionally layers the target slot's
// per-transaction diffs up to (but not including) `index`. Returns
// [pending.ErrPreConfirmedNotFound] if blockNumber isn't in the chain, or
// [pending.ErrTransactionIndexOutOfBounds] if `index` exceeds the target's
// transaction count.
func (c *ChainReader) PreConfirmedStateBeforeIndexAt(
	blockNumber uint64,
	index uint,
	bcReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	if !c.contains(blockNumber) {
		return nil, nil, pending.ErrPreConfirmedNotFound
	}

	stateDiff := core.EmptyStateDiff()
	var target *pending.PreConfirmed
	var newClasses map[felt.Felt]core.ClassDefinition
	for entry := range c.OldestFirst() {
		if entry.Block.Number == blockNumber {
			target = entry
			break
		}
		stateDiff.Merge(entry.StateUpdate.StateDiff)
		newClasses = mergeClassesInto(newClasses, entry.NewClasses)
	}
	// Invariant: blockNumber passed the range check, so a contiguous chain must
	// contain it. A nil target means the chain has a gap (a bug), not a miss.
	if target == nil {
		return nil, nil, fmt.Errorf(
			"pre-confirmed chain invariant broken: block %d within [%d, %d] but no matching entry",
			blockNumber, c.oldestPreConf(), c.tip(),
		)
	}
	if index > uint(len(target.Block.Transactions)) {
		return nil, nil, pending.ErrTransactionIndexOutOfBounds
	}
	newClasses = mergeClassesInto(newClasses, target.NewClasses)
	base, closer, err := c.baseState(bcReader)
	if err != nil {
		return nil, nil, err
	}
	for _, txStateDiff := range target.TransactionStateDiffs[:index] {
		stateDiff.Merge(txStateDiff)
	}
	return pending.NewState(&stateDiff, newClasses, base, blockNumber), closer, nil
}

// baseState opens the canonical state immediately below the chain's oldest
// slot. Caller must hold a non-empty chain (length>0 verified upstream by the
// public methods).
func (c *ChainReader) baseState(
	bcReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	return bcReader.StateAtBlockNumber(c.oldestPreConf() - 1)
}

// contains returns true when a non-empty chain reader contains a block with `blockNum`
func (c *ChainReader) contains(blockNum uint64) bool {
	return c.length > 0 && (blockNum >= c.oldestPreConf() && blockNum <= c.tip())
}

// tip returns the most recent block number contained in the ChainReader
func (c *ChainReader) tip() uint64 {
	return c.head.preconfirmed.Block.Number
}

// oldestPreConf returns the oldest block number contained in the ChainReader
func (c *ChainReader) oldestPreConf() uint64 {
	return c.head.preconfirmed.Block.Number - uint64(c.length-1)
}

// walkOldestFirst recurses to the oldest entry of the chain and yields entries on
// the way back up, producing oldest-first iteration order without
// materialising a slice. remaining bounds the depth so head-aligned views
// stop short of the underlying linked list's nil terminator (relevant after
// SnapshotForBlock trims entries at or below the canonical head). Returns false
// when the yield callback aborts iteration. Depth equals the caller's Length.
func walkOldestFirst(
	current *node,
	remaining int,
	yield func(*pending.PreConfirmed) bool,
) bool {
	if current == nil || remaining == 0 {
		return true
	}
	if !walkOldestFirst(current.parent, remaining-1, yield) {
		return false
	}
	return yield(current.preconfirmed)
}

// ChainStorage holds a contiguous run of pre-confirmed blocks above the
// canonical head. Readers obtain a head-aligned view via [SnapshotForBlock].
// Single writer (polling loop) with many concurrent readers; reads are
// lock-free via atomic.Pointer.
type ChainStorage struct {
	inner atomic.Pointer[ChainReader]
}

func NewChainStorage() *ChainStorage {
	return &ChainStorage{}
}

// SnapshotForBlock returns a head-aligned view of the pre-confirmed chain based
// on the input `blockNumber`. If the `blockNumber` sits outside the pre-confirmed chain
// range, an empty chain is returned. Otherwise, a new chain is returned in the
// [blockNumber, chain.tip()] range.
//
// The total size of the pre-confirmed chain is uncapped and it can exceed
// [core.BlockHashLag], causing execution simulations on longer chain to fail due to this.
func (s *ChainStorage) SnapshotForBlock(blockNumber uint64) ChainReader {
	current := s.inner.Load()
	if current == nil || !current.contains(blockNumber) {
		return ChainReader{}
	}

	want := int(current.tip() - blockNumber + 1)
	return ChainReader{head: current.head, length: want}
}

// ApplyUpdate atomically evolves the stored chain from a wire-side update.
// blockNumber is the height targeted by the update; baseTxCount is the
// knownTransactionCount the poller sent (consulted only for the Delta case
// as a defensive race-check against the targeted slot). oldestPreConf is the
// slot the chain's oldest entry is expected to occupy: the first pre-confirmed
// slot above the canonical head, head number+1. Returns the affected entry, or
// nil when the update was a no-op (NoChange, preserved, rejected at cap, etc.).
//
// On CAS failure the chain changed between Load and CompareAndSwap; we return
// an error instead of retrying.
//
// A NoChange still registers newClasses on the targeted (tip) slot when non-empty: a
// re-poll that reported no content change may still carry declared classes we only just
// fetched for that block.
func (s *ChainStorage) ApplyUpdate(
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	oldestPreConf uint64,
	newClasses map[felt.Felt]core.ClassDefinition,
) (*pending.PreConfirmed, error) {
	current := s.inner.Load()
	newChain, affected, err := computeUpdate(
		current,
		update,
		blockNumber,
		baseTxCount,
		oldestPreConf,
		newClasses,
	)
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

// mergeClassesCopying returns base with the entries of extra added, without mutating base's map
// (copy-on-write). Returns base unchanged when extra is empty.
func mergeClassesCopying(
	base, extra map[felt.Felt]core.ClassDefinition,
) map[felt.Felt]core.ClassDefinition {
	if len(extra) == 0 {
		return base
	}
	merged := maps.Clone(base)
	if merged == nil {
		merged = make(map[felt.Felt]core.ClassDefinition, len(extra))
	}
	maps.Copy(merged, extra)
	return merged
}

// AdvanceTo realigns the chain to a new canonical head, given as oldestPreConf —
// the slot the chain's oldest entry must occupy after the realignment, i.e.
// the first pre-confirmed slot above that head (head number+1). Three
// outcomes:
//
//   - oldestPreConf == the chain's current oldest: already aligned, no-op.
//   - oldestPreConf > mostRecent (head advanced past everything we stored) OR
//     oldestPreConf < the current oldest (head reverted below us — every entry's
//     parent now references a discarded block): drop the whole chain. The
//     next poll bootstraps fresh against the new head.
//   - current oldest < oldestPreConf <= mostRecent: rebuild from the new oldest
//     slot up so the surviving nodes nil-terminate cleanly and the dropped
//     tail is GC-able.
//
// Pre-pop readers retain their *ChainReader and walk the original (still
// intact) nodes; the new chain references only fresh nodes.
//
// Single-writer: like ApplyUpdate, this assumes the pre-confirmed poller
// goroutine is the only writer.
func (s *ChainStorage) AdvanceTo(oldestPreConf uint64) bool {
	current := s.inner.Load()
	if current == nil || current.length == 0 {
		return false
	}

	currentOldest := current.oldestPreConf()
	if oldestPreConf == currentOldest {
		return false
	}

	if !current.contains(oldestPreConf) {
		return s.inner.CompareAndSwap(current, nil)
	}

	drop := int(oldestPreConf - currentOldest)
	keep := current.length - drop
	newChain := &ChainReader{head: rebuild(current.head, keep), length: keep}

	return s.inner.CompareAndSwap(current, newChain)
}

// rebuild walks `keep` levels down from n via parent pointers, then on the
// way back up builds fresh nodes so the oldest node has parent==nil. The
// original nodes stay reachable for any concurrent walkers of the old chain
// pointer; once those release, the dropped tail (below the new oldest slot)
// becomes unreachable and GC-collectable.
func rebuild(current *node, keep int) *node {
	if keep == 0 || current == nil {
		return nil
	}
	child := rebuild(current.parent, keep-1)
	return &node{preconfirmed: current.preconfirmed, parent: child}
}

// computeUpdate is the pure dispatcher that turns a wire-side update into a
// new chain. Four mutually-exclusive cases:
//
//   - empty chain                       → bootstrapChain (only PreConfirmedBlock accepted)
//   - blockNumber > mostRecent + 1     → gap above tip, reject (no-op return)
//   - blockNumber == mostRecent + 1    → appendMostRecent (extension by one)
//   - blockNumber within [oldest, mostRecent] → replaceSlot (in-chain mutation)
//
// Updates below the oldest slot and unalignment with oldestPreConf are rejected
// here too — callers must AdvanceTo first to align the chain to a fresh head.
//
// Returns (newChain, affected, err): newChain==nil means "no-op, leave the
// store as-is"; err is reserved for invariant violations (e.g. the chain's
// oldest slot drifted from oldestPreConf, delta baseTxCount mismatch).
func computeUpdate(
	current *ChainReader,
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	oldestPreConf uint64,
	newClasses map[felt.Felt]core.ClassDefinition,
) (*ChainReader, *pending.PreConfirmed, error) {
	if current == nil || current.length == 0 {
		block, ok := update.(starknet.PreConfirmedBlock)
		if !ok {
			return nil, nil, fmt.Errorf("bootstrap rejected: want PreConfirmedBlock, got %T", update)
		}
		return bootstrapChain(&block, blockNumber, oldestPreConf, newClasses)
	}

	currentOldest := current.oldestPreConf()
	if currentOldest != oldestPreConf {
		return nil, nil, fmt.Errorf(
			"chain's oldest pre-confirmed slot %d not aligned with expected %d",
			currentOldest, oldestPreConf,
		)
	}

	// In-chain update — locate the target slot. blockNumber below the oldest
	// slot means the apply target is at or below the canonical head
	// (oldestPreConf == head+1), i.e. already committed; the caller is asking us
	// to write into the past.
	if blockNumber < currentOldest {
		return nil, nil, fmt.Errorf(
			"applying target %d below the oldest pre-confirmed slot %d", blockNumber, currentOldest,
		)
	}

	// Gap above tip — should never happen under a well-behaved poller, which
	// backfills intermediate heights before applying the latest. Surface as
	// an error so the bug isn't masked as a silent no-op.
	tip := current.tip()
	if blockNumber > tip+1 {
		return nil, nil, fmt.Errorf(
			"gap above tip: block %d > mostRecent+1 %d", blockNumber, tip+1,
		)
	}

	// Append as new most recent. Only PreConfirmedBlock makes sense at a
	// brand-new slot; a Delta would have nothing to merge into.
	if blockNumber == tip+1 {
		block, ok := update.(starknet.PreConfirmedBlock)
		if !ok {
			return nil, nil, fmt.Errorf(
				"append rejected at slot %d: want PreConfirmedBlock, got %T", blockNumber, update,
			)
		}
		return extend(current, &block, blockNumber, newClasses)
	}

	return replaceSlot(current, update, blockNumber, baseTxCount, newClasses)
}

// bootstrapChain handles the first entry case (empty storage). The caller
// (computeUpdate) has already narrowed the update variant to PreConfirmedBlock.
// The precondition is blockNumber == oldestPreConf, the first pre-confirmed slot
// above the canonical head (head number+1). Returns the new length-1 chain
// plus the adapted entry.
func bootstrapChain(
	block *starknet.PreConfirmedBlock,
	blockNumber uint64,
	oldestPreConf uint64,
	newClasses map[felt.Felt]core.ClassDefinition,
) (*ChainReader, *pending.PreConfirmed, error) {
	if blockNumber != oldestPreConf {
		return nil, nil, fmt.Errorf(
			"bootstrap block %d invalid: oldest pre-confirmed slot is %d", blockNumber, oldestPreConf,
		)
	}
	next, err := sn2core.AdaptPreConfirmedBlock(block, blockNumber)
	if err != nil {
		return nil, nil, err
	}
	if err := core.CheckBlockVersion(next.Block.ProtocolVersion); err != nil {
		return nil, nil, err
	}
	next.NewClasses = newClasses
	newNode := &node{preconfirmed: &next, parent: nil}
	return &ChainReader{head: newNode, length: 1}, &next, nil
}

// extend grows the chain by one when the incoming block's blockNumber equals
// mostRecent+1.
func extend(
	current *ChainReader,
	block *starknet.PreConfirmedBlock,
	blockNumber uint64,
	newClasses map[felt.Felt]core.ClassDefinition,
) (*ChainReader, *pending.PreConfirmed, error) {
	next, err := sn2core.AdaptPreConfirmedBlock(block, blockNumber)
	if err != nil {
		return nil, nil, err
	}
	if err := core.CheckBlockVersion(next.Block.ProtocolVersion); err != nil {
		return nil, nil, err
	}
	next.NewClasses = newClasses
	newNode := &node{preconfirmed: &next, parent: current.head}
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
//   - PreConfirmedNoChange — content is unchanged; only merges newClasses into the
//     existing tip. A no-op (nil, nil, nil) when the tip already holds them all.
//
// Returns (nil, nil, nil) when shouldPreserveSlot says "keep the existing slot,
// no broadcast needed." Caller must have already validated that blockNumber
// falls within [oldest, mostRecent].
func replaceSlot(
	current *ChainReader,
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	newClasses map[felt.Felt]core.ClassDefinition,
) (*ChainReader, *pending.PreConfirmed, error) {
	depthFromHead := int(current.tip() - blockNumber)
	target := current.head
	for range depthFromHead {
		target = target.parent
	}

	switch variant := update.(type) {
	case starknet.PreConfirmedBlock:
		next, err := sn2core.AdaptPreConfirmedBlock(&variant, blockNumber)
		if err != nil {
			return nil, nil, err
		}
		if err := core.CheckBlockVersion(next.Block.ProtocolVersion); err != nil {
			return nil, nil, err
		}
		next.NewClasses = newClasses
		if shouldPreserveSlot(target.preconfirmed, &next) {
			return nil, nil, nil
		}
		newNode := &node{preconfirmed: &next, parent: target.parent}
		return &ChainReader{
			head:   newNode,
			length: current.length - depthFromHead,
		}, &next, nil

	case starknet.PreConfirmedDeltaUpdate:
		// Delta updates can only target the chain tip
		if depthFromHead != 0 {
			return nil, nil, fmt.Errorf("delta at non-tip slot %d (depth %d)", blockNumber, depthFromHead)
		}
		if uint64(len(target.preconfirmed.Block.Transactions)) != baseTxCount {
			return nil, nil, ErrBaseTxCountMismatch
		}
		next, err := sn2core.AdaptPreConfirmedWithDelta(target.preconfirmed, &variant)
		if err != nil {
			return nil, nil, err
		}
		next.NewClasses = mergeClassesCopying(next.NewClasses, newClasses)
		newNode := &node{preconfirmed: &next, parent: target.parent}
		return &ChainReader{
			head:   newNode,
			length: current.length,
		}, &next, nil

	case starknet.PreConfirmedNoChange:
		// A NoChange branch exists only to register newly-fetched classes; with none there
		// is nothing to do, whatever slot it targets.
		if len(newClasses) == 0 {
			return nil, nil, nil
		}
		// NoChange only ever targets the tip: the poller re-polls a slot only while
		// it is the most recent, so a NoChange below the tip is a bug.
		if depthFromHead != 0 {
			return nil, nil, fmt.Errorf(
				"no-change at non-tip slot %d (depth %d)", blockNumber, depthFromHead,
			)
		}
		merged := mergeClassesCopying(target.preconfirmed.NewClasses, newClasses)
		if len(merged) == len(target.preconfirmed.NewClasses) {
			return nil, nil, nil // tip already holds them all
		}
		next := *target.preconfirmed
		next.NewClasses = merged
		newNode := &node{preconfirmed: &next, parent: target.parent}
		return &ChainReader{head: newNode, length: current.length}, &next, nil
	}
	return nil, nil, fmt.Errorf("unknown PreConfirmedUpdate variant %T", update)
}

// shouldPreserveSlot keeps the existing slot when the incoming pre-confirmed is
// at the same identifier with no extra transactions, or carries the blank
// placeholder identifier. A different real identifier (new round), a richer
// same-identifier block, or one carrying declared classes the existing slot lacks
// replaces.
func shouldPreserveSlot(existing, incoming *pending.PreConfirmed) bool {
	if incoming.BlockIdentifier != existing.BlockIdentifier &&
		incoming.BlockIdentifier != feeder.PreConfirmedBlankIdentifier {
		return false
	}
	if incoming.Block.TransactionCount > existing.Block.TransactionCount {
		return false
	}

	if len(incoming.NewClasses) > len(existing.NewClasses) {
		return false
	}
	return true
}

// mergeClassesInto copies src's entries into dst, mutating dst in place and
// returning it. When dst is nil it returns a fresh clone of src.
func mergeClassesInto(
	dst, src map[felt.Felt]core.ClassDefinition,
) map[felt.Felt]core.ClassDefinition {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		return maps.Clone(src)
	}
	maps.Copy(dst, src)
	return dst
}
