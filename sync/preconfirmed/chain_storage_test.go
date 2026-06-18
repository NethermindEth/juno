package preconfirmed_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ---- helpers --------------------------------------------------------------

func headAt(n uint64) *core.Header { return &core.Header{Number: n} }

// roundID is the per-slot identifier convention used across the storage
// tests: slot N's block carries identifier "round-N", so each slot in a
// chain is structurally distinguishable from every other (and a slot-mixup
// bug would surface as an identifier mismatch in assertChain).
func roundID(slot uint64) string { return fmt.Sprintf("round-%d", slot) }

// chainBlockNumbers collects block numbers in oldest-first order. Used where
// the test cares about ordering / contiguity rather than per-slot identity
// (concurrent reader, iterator direction).
func chainBlockNumbers(c *preconfirmed.ChainReader) []uint64 {
	if c == nil {
		return nil
	}
	out := make([]uint64, 0, c.Length())
	for pc := range c.OldestFirst() {
		out = append(out, pc.Block.Number)
	}
	return out
}

// applyBlock constructs a synthetic PreConfirmedBlock, applies it to storage,
// and returns it so callers can hand it to entry() for assertChain.
func applyBlock(
	t *testing.T,
	s *preconfirmed.ChainStorage,
	identifier string,
	txCount int,
	number uint64,
	head *core.Header,
) starknet.PreConfirmedBlock {
	t.Helper()
	block := makeTestPreConfirmedBlock(identifier, txCount)
	_, err := s.ApplyUpdate(block, number, 0, head)
	require.NoError(t, err)
	return block
}

// rangeEntries returns `entry(from+i, &blocks[i])` for each block, suitable
// for spread-passing to assertChain.
func rangeEntries(from uint64, blocks []starknet.PreConfirmedBlock) []expectedEntry {
	out := make([]expectedEntry, len(blocks))
	for i := range blocks {
		out[i] = entry(from+uint64(i), &blocks[i])
	}
	return out
}

// ---- TestChainStorageApplyUpdate ------------------------------------------

func TestChainStorageApplyUpdate(t *testing.T) {
	t.Run("bootstrap: rejects Delta on empty chain", testApplyUpdateBootstrapRejectsDelta)
	t.Run("bootstrap: NoChange on empty is a no-op", testApplyUpdateBootstrapNoChangeNoop)
	t.Run("bootstrap: rejected at wrong height", testApplyUpdateBootstrapWrongHeight)
	t.Run("bootstrap: accepted at head+1", testApplyUpdateBootstrapAtHeadPlusOne)
	t.Run("bootstrap: accepted at genesis when head is nil", testApplyUpdateBootstrapAtGenesis)
	t.Run(
		"bootstrap: rejected at non-zero height when nil head",
		testApplyUpdateBootstrapNonzeroAtNilHead,
	)
	t.Run("extend: gap above tip is rejected", testApplyUpdateExtendGapRejected)
	t.Run("extend: non-Block update at brand-new slot rejected", testApplyUpdateExtendNonBlockRejected)
	t.Run(
		"replace-tip: same identity non-richer is preserved",
		testApplyUpdateReplaceTipNonRicherPreserved,
	)
	t.Run(
		"replace-tip: same identifier richer block replaces",
		testApplyUpdateReplaceTipRicherReplaces,
	)
	t.Run("replace-tip: new round at most recent replaces", testApplyUpdateReplaceTipNewRound)
	t.Run("replace-tip: blank identifier never overrides a real round",
		testApplyUpdateReplaceTipBlankIgnored)
	t.Run("reorg: new round at non-tip truncates above", testApplyUpdateReorgNonTipTruncates)
	t.Run("reorg: new round at bottom slot truncates above", testApplyUpdateReorgBottomTruncates)
	t.Run("reorg: re-extend after reorg rebuilds the chain", testApplyUpdateReorgReExtend)
	t.Run("reorg: sequential reorgs at depths each truncate above", testApplyUpdateReorgSequential)
	t.Run("reorg: pre-reorg snapshot walks the old chain", testApplyUpdateReorgPreSnapshotIntact)
	t.Run("replace-tip: delta swaps tip with a fresh node carrying merged txs",
		testApplyUpdateDeltaAtTip)
	t.Run("delta: at non-tip is rejected", testApplyUpdateDeltaAtNonTipRejected)
	t.Run("delta: wrong baseTxCount returns mismatch err", testApplyUpdateDeltaWrongBaseTxCount)
	t.Run(
		"delta: mismatched identifier is rejected by adapter",
		testApplyUpdateDeltaIdentifierMismatch,
	)
	t.Run("apply below chain bottom is rejected", testApplyUpdateBelowBottomRejected)
}

func testApplyUpdateBootstrapRejectsDelta(t *testing.T) {
	head := headAt(100)
	s := preconfirmed.NewChainStorage()
	pc, err := s.ApplyUpdate(starknet.PreConfirmedDeltaUpdate{}, 101, 0, head)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bootstrap rejected")
	require.Nil(t, pc)
	view := s.SnapshotForHead(head)
	require.Zero(t, view.Length())
}

func testApplyUpdateBootstrapNoChangeNoop(t *testing.T) {
	head := headAt(100)
	s := preconfirmed.NewChainStorage()
	pc, err := s.ApplyUpdate(starknet.PreConfirmedNoChange{}, 101, 0, head)
	require.NoError(t, err)
	require.Nil(t, pc)
	view := s.SnapshotForHead(head)
	require.Zero(t, view.Length())
}

func testApplyUpdateBootstrapWrongHeight(t *testing.T) {
	head := headAt(100)
	s := preconfirmed.NewChainStorage()
	_, err := s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(103), 0), 103, 0, head)
	require.Error(t, err)
	view := s.SnapshotForHead(head)
	require.Zero(t, view.Length())
}

func testApplyUpdateBootstrapAtHeadPlusOne(t *testing.T) {
	head := headAt(100)
	s := preconfirmed.NewChainStorage()
	b := applyBlock(t, s, roundID(101), 1, 101, head)
	view := s.SnapshotForHead(head)
	assertChain(t, &view, entry(101, &b))
}

func testApplyUpdateBootstrapAtGenesis(t *testing.T) {
	s := preconfirmed.NewChainStorage()
	b := applyBlock(t, s, roundID(0), 0, 0, nil)
	view := s.SnapshotForHead(nil)
	assertChain(t, &view, entry(0, &b))
}

func testApplyUpdateBootstrapNonzeroAtNilHead(t *testing.T) {
	s := preconfirmed.NewChainStorage()
	_, err := s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(1), 0), 1, 0, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "genesis")
	view := s.SnapshotForHead(nil)
	require.Zero(t, view.Length())
}

func testApplyUpdateExtendGapRejected(t *testing.T) {
	head := headAt(100)
	s := preconfirmed.NewChainStorage()
	b101 := applyBlock(t, s, roundID(101), 0, 101, head)
	b102 := applyBlock(t, s, roundID(102), 0, 102, head)
	before := s.SnapshotForHead(head)

	// Skip slot 103, attempt to apply at 104.
	_, err := s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(104), 0), 104, 0, head)
	require.Error(t, err)
	require.Contains(t, err.Error(), "gap above tip")
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head(), "chain pointer must be unchanged on error")
	assertChain(t, &after, entry(101, &b101), entry(102, &b102))
}

func testApplyUpdateExtendNonBlockRejected(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	seed := applyBlock(t, s, roundID(1), 0, 1, head)
	before := s.SnapshotForHead(head)

	// A Delta at brand-new slot 2 is rejected before identifier validation —
	// only PreConfirmedBlock is valid at a new tip.
	delta := makeTestDelta(roundID(2), 1)
	_, err := s.ApplyUpdate(delta, 2, 0, head)
	require.Error(t, err)
	require.Contains(t, err.Error(), "append rejected")
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &seed))
}

func testApplyUpdateReplaceTipNonRicherPreserved(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 1, 1, head)
	b2 := applyBlock(t, s, roundID(2), 1, 2, head)
	before := s.SnapshotForHead(head)

	// Re-apply at slot 2 with matching identifier and same tx count → preserved.
	applyBlock(t, s, roundID(2), 1, 2, head)
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &b1), entry(2, &b2))
}

func testApplyUpdateReplaceTipRicherReplaces(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 1, 1, head)
	applyBlock(t, s, roundID(2), 1, 2, head)
	before := s.SnapshotForHead(head)

	// Re-apply at slot 2 with matching identifier but more txs → replaces.
	bRicher := applyBlock(t, s, roundID(2), 3, 2, head)
	after := s.SnapshotForHead(head)
	require.NotSame(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &b1), entry(2, &bRicher))
}

func testApplyUpdateReplaceTipNewRound(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 1, 1, head)
	applyBlock(t, s, roundID(2), 1, 2, head)
	before := s.SnapshotForHead(head)

	// Different identifier at slot 2 → new round replaces.
	bNew := applyBlock(t, s, "round-2-alt", 0, 2, head)
	after := s.SnapshotForHead(head)
	require.NotSame(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &b1), entry(2, &bNew))
}

func testApplyUpdateReplaceTipBlankIgnored(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 1, 1, head)
	b2 := applyBlock(t, s, roundID(2), 1, 2, head)
	before := s.SnapshotForHead(head)

	applyBlock(t, s, feeder.PreConfirmedBlankIdentifier, 0, 2, head)
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &b1), entry(2, &b2))
}

func testApplyUpdateReorgNonTipTruncates(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 0, 1, head)
	applyBlock(t, s, roundID(2), 0, 2, head)
	applyBlock(t, s, roundID(3), 0, 3, head)

	// New round at non-tip slot 2 → slot 3 is truncated.
	bReorg := applyBlock(t, s, "round-2-alt", 5, 2, head)
	view := s.SnapshotForHead(head)
	assertChain(t, &view, entry(1, &b1), entry(2, &bReorg))
}

func testApplyUpdateReorgBottomTruncates(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= 4; n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}

	// Reorg at slot 1 (bottom) with a new round — everything above truncated.
	bReorg := applyBlock(t, s, "round-1-alt", 2, 1, head)
	view := s.SnapshotForHead(head)
	assertChain(t, &view, entry(1, &bReorg))
}

func testApplyUpdateReorgReExtend(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 0, 1, head)
	applyBlock(t, s, roundID(2), 0, 2, head) // will be replaced
	applyBlock(t, s, roundID(3), 0, 3, head) // will be truncated

	// Reorg at slot 2.
	b2Alt := applyBlock(t, s, "round-2-alt", 1, 2, head)
	view := s.SnapshotForHead(head)
	assertChain(t, &view, entry(1, &b1), entry(2, &b2Alt))

	// Re-extend slots 3 and 4 with new rounds.
	b3Alt := applyBlock(t, s, "round-3-alt", 0, 3, head)
	b4Alt := applyBlock(t, s, "round-4-alt", 0, 4, head)
	view2 := s.SnapshotForHead(head)
	assertChain(t, &view2,
		entry(1, &b1),
		entry(2, &b2Alt),
		entry(3, &b3Alt),
		entry(4, &b4Alt),
	)
}

func testApplyUpdateReorgSequential(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	b1 := applyBlock(t, s, roundID(1), 0, 1, head)
	b2 := applyBlock(t, s, roundID(2), 0, 2, head)
	b3 := applyBlock(t, s, roundID(3), 0, 3, head)
	applyBlock(t, s, roundID(4), 0, 4, head)
	applyBlock(t, s, roundID(5), 0, 5, head)

	// First reorg at slot 4.
	b4Alt := applyBlock(t, s, "round-4-alt", 0, 4, head)
	view := s.SnapshotForHead(head)
	assertChain(t, &view,
		entry(1, &b1),
		entry(2, &b2),
		entry(3, &b3),
		entry(4, &b4Alt),
	)

	// Re-extend slot 5 with a new round.
	b5Alt := applyBlock(t, s, "round-5-alt", 0, 5, head)
	view2 := s.SnapshotForHead(head)
	assertChain(t, &view2,
		entry(1, &b1),
		entry(2, &b2),
		entry(3, &b3),
		entry(4, &b4Alt),
		entry(5, &b5Alt),
	)

	// Second reorg at slot 3 — truncates everything above.
	b3Alt := applyBlock(t, s, "round-3-alt", 0, 3, head)
	view3 := s.SnapshotForHead(head)
	assertChain(t, &view3,
		entry(1, &b1),
		entry(2, &b2),
		entry(3, &b3Alt),
	)
}

func testApplyUpdateReorgPreSnapshotIntact(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	original := make([]starknet.PreConfirmedBlock, 4)
	for i := range original {
		n := uint64(i + 1)
		original[i] = applyBlock(t, s, roundID(n), 0, n, head)
	}
	pre := s.SnapshotForHead(head)

	// Reorg at slot 2 — slots 3 and 4 truncated in the live chain.
	b2Alt := applyBlock(t, s, "round-2-alt", 0, 2, head)
	view := s.SnapshotForHead(head)
	assertChain(t, &view, entry(1, &original[0]), entry(2, &b2Alt))

	// Pre-reorg snapshot must still walk the original four-entry chain.
	assertChain(t, &pre, rangeEntries(1, original)...)
}

func testApplyUpdateDeltaAtTip(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	// Delta and seed MUST share an identifier for the merge to be valid.
	const round = "round-1"
	seed := applyBlock(t, s, round, 2, 1, head)

	delta := makeTestDelta(round, 3)
	_, err := s.ApplyUpdate(delta, 1, 2, head)
	require.NoError(t, err)

	// 2 base + 3 appended via delta.
	view := s.SnapshotForHead(head)
	assertChain(t, &view, entry(1, &seed, &delta))
}

func testApplyUpdateDeltaAtNonTipRejected(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	const slot1Round = "round-1"
	applyBlock(t, s, slot1Round, 1, 1, head)
	applyBlock(t, s, roundID(2), 0, 2, head)
	applyBlock(t, s, roundID(3), 0, 3, head)
	before := s.SnapshotForHead(head)

	delta := makeTestDelta(slot1Round, 2)
	_, err := s.ApplyUpdate(delta, 1, 1, head)
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-tip")
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head(), "rejected delta must not mutate storage")
}

func testApplyUpdateDeltaWrongBaseTxCount(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	const round = "round-1"
	seed := applyBlock(t, s, round, 2, 1, head)
	before := s.SnapshotForHead(head)

	// Matching identifier — failure is purely from the baseTxCount race-check.
	delta := makeTestDelta(round, 1)
	_, err := s.ApplyUpdate(delta, 1, 99, head)
	require.ErrorIs(t, err, preconfirmed.ErrBaseTxCountMismatch)
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &seed))
}

func testApplyUpdateDeltaIdentifierMismatch(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	seed := applyBlock(t, s, "round-1", 1, 1, head)
	before := s.SnapshotForHead(head)

	delta := makeTestDelta("round-1-different", 1)
	_, err := s.ApplyUpdate(delta, 1, 1, head)
	require.Error(t, err)
	after := s.SnapshotForHead(head)
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(1, &seed))
}

func testApplyUpdateBelowBottomRejected(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	applyBlock(t, s, roundID(1), 0, 1, head)
	b2 := applyBlock(t, s, roundID(2), 0, 2, head)
	// Advance head past slot 1; chain bottom is now 2.
	s.AdvanceTo(headAt(1))
	before := s.SnapshotForHead(headAt(1))

	// Identifier is irrelevant here — the below-bottom check fires first.
	delta := makeTestDelta(roundID(1), 1)
	_, err := s.ApplyUpdate(delta, 1, 0, headAt(1))
	require.Error(t, err, "apply target below bottom must surface as an error")
	after := s.SnapshotForHead(headAt(1))
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(2, &b2))
}

// ---- TestChainStorageAdvanceTo --------------------------------------------

func TestChainStorageAdvanceTo(t *testing.T) {
	t.Run("partial drop preserves pre-advance snapshot", testAdvanceToPartialDrop)
	t.Run("full drop clears the chain", testAdvanceToFullDrop)
	t.Run(
		"canonical reorg clears chain and recovers on next poll",
		testAdvanceToReorgClearsAndRecovers,
	)
	t.Run("nil head with bootstrapped chain is a no-op", testAdvanceToNilHeadBootstrapped)
}

// testAdvanceToNilHeadBootstrapped mirrors the poller's genesis tick: head is
// nil because blockchain.HeadsHeader returned ErrKeyNotFound, but a prior tick
// already bootstrapped block 0. AdvanceTo(nil) must treat nil as "head below
// every slot" and leave the chain intact.
func testAdvanceToNilHeadBootstrapped(t *testing.T) {
	s := preconfirmed.NewChainStorage()
	b := applyBlock(t, s, roundID(0), 0, 0, nil)
	before := s.SnapshotForHead(nil)

	require.NotPanics(t, func() { s.AdvanceTo(nil) })

	after := s.SnapshotForHead(nil)
	require.Same(t, before.Head(), after.Head())
	assertChain(t, &after, entry(0, &b))
}

func testAdvanceToPartialDrop(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	blocks := make([]starknet.PreConfirmedBlock, 5)
	for i := range blocks {
		n := uint64(i + 1)
		blocks[i] = applyBlock(t, s, roundID(n), 0, n, head)
	}
	preAdvance := s.SnapshotForHead(head)
	all := rangeEntries(1, blocks)
	assertChain(t, &preAdvance, all...)

	// Head advances by 2 — blocks 1 and 2 are now committed.
	s.AdvanceTo(headAt(2))

	view := s.SnapshotForHead(headAt(2))
	assertChain(t, &view, rangeEntries(3, blocks[2:])...)
	// Pre-advance snapshot is untouched (rebuild, not mutation).
	assertChain(t, &preAdvance, all...)
}

func testAdvanceToFullDrop(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= 3; n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}
	s.AdvanceTo(headAt(10))
	view := s.SnapshotForHead(headAt(10))
	require.Zero(t, view.Length())
}

// testAdvanceToReorgClearsAndRecovers simulates a canonical head reorg: the
// chain was built on top of head=5 (entries at slots 6,7), then the canonical
// head reverts to 3. Every stored entry's parent now references a discarded
// block, so AdvanceTo must drop the whole chain. SnapshotForHead at the new
// head reports empty (callers see no pre_confirmed), and the next poll
// (applying a fresh block at the new head+1) bootstraps cleanly.
func testAdvanceToReorgClearsAndRecovers(t *testing.T) {
	oldHead := headAt(5)
	s := preconfirmed.NewChainStorage()
	applyBlock(t, s, roundID(6), 0, 6, oldHead)
	applyBlock(t, s, roundID(7), 0, 7, oldHead)
	preReorg := s.SnapshotForHead(oldHead)
	require.Equal(t, 2, preReorg.Length())

	// Reorg: canonical head reverts from 5 to 3.
	newHead := headAt(3)
	s.AdvanceTo(newHead)

	// Storage cleared; readers see nothing for the new head.
	cleared := s.SnapshotForHead(newHead)
	require.Zero(t, cleared.Length())
	view := s.SnapshotForHead(newHead)
	require.Zero(t, view.Length())
	require.Nil(t, view.Head())

	// Next poll lands a fresh pre_confirmed at the new head+1; chain recovers.
	b4 := applyBlock(t, s, roundID(4), 0, 4, newHead)
	recovered := s.SnapshotForHead(newHead)
	assertChain(t, &recovered, entry(4, &b4))
}

// ---- TestChainStorageSnapshotForHead --------------------------------------

func TestChainStorageSnapshotForHead(t *testing.T) {
	t.Run("empty storage returns zero-value view", testSnapshotForHeadEmpty)
	t.Run("trims view when storage is briefly stale", testSnapshotForHeadStaleTrim)
	t.Run("zero-value when head+1 is above most recent", testSnapshotForHeadEmptyAboveTip)
}

func testSnapshotForHeadEmpty(t *testing.T) {
	s := preconfirmed.NewChainStorage()
	view := s.SnapshotForHead(headAt(100))
	require.Zero(t, view.Length())
	require.Nil(t, view.Head())
}

func testSnapshotForHeadStaleTrim(t *testing.T) {
	// Bootstrap under head=0 → chain bottom=1. Then a reader passes head=2
	// before AdvanceTo has run.
	storageHead := headAt(0)
	s := preconfirmed.NewChainStorage()
	blocks := make([]starknet.PreConfirmedBlock, 5)
	for i := range blocks {
		n := uint64(i + 1)
		blocks[i] = applyBlock(t, s, roundID(n), 0, n, storageHead)
	}

	stale := s.SnapshotForHead(headAt(2))
	assertChain(t, &stale, rangeEntries(3, blocks[2:])...)

	// Stored chain is unchanged; only the view's length was trimmed.
	full := s.SnapshotForHead(storageHead)
	assertChain(t, &full, rangeEntries(1, blocks)...)
}

func testSnapshotForHeadEmptyAboveTip(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= 5; n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}
	view := s.SnapshotForHead(headAt(99))
	require.Zero(t, view.Length())
}

// ---- TestChainStorageSnapshot ---------------------------------------------

func TestChainStorageSnapshot(t *testing.T) {
	t.Run("empty returns zero-value view", testSnapshotEmpty)
	t.Run("survives subsequent updates", testSnapshotSurvivesUpdates)
}

func testSnapshotEmpty(t *testing.T) {
	s := preconfirmed.NewChainStorage()
	view := s.SnapshotForHead(headAt(0))
	require.Zero(t, view.Length())
}

func testSnapshotSurvivesUpdates(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	blocks := make([]starknet.PreConfirmedBlock, 4)
	for i := range blocks {
		n := uint64(i + 1)
		blocks[i] = applyBlock(t, s, roundID(n), 0, n, head)
	}
	view := s.SnapshotForHead(head)

	// Drive subsequent updates: append at slot 5, richer-replace at slot 5
	// (matching identifier required for richer-replace), tail-pop.
	applyBlock(t, s, roundID(5), 0, 5, head)
	applyBlock(t, s, roundID(5), 7, 5, head)
	s.AdvanceTo(headAt(2))

	// The snapshot taken pre-mutation still walks the original chain at
	// the original (pre-richer) tx counts.
	assertChain(t, &view, rangeEntries(1, blocks)...)
}

// ---- TestChainReader ------------------------------------------------------

func TestChainReader(t *testing.T) {
	t.Run("NewestFirst yields newest-to-oldest", testChainReaderNewestFirstOrder)
	t.Run("OldestFirst yields oldest-to-newest", testChainReaderOldestFirstOrder)
	t.Run("NewestFirst early-exit stops walking", testChainReaderNewestFirstEarlyExit)
	t.Run("OldestFirst early-exit stops walking", testChainReaderOldestFirstEarlyExit)
	t.Run("iterators are alloc-free", testChainReaderIteratorsAllocFree)
	t.Run("PreConfirmedStateAt composes diffs through target block",
		testChainReaderPreConfirmedStateAtComposes)
	t.Run("PreConfirmedStateAt rejects blockNumber outside chain",
		testChainReaderPreConfirmedStateAtOutOfRange)
	t.Run("PreConfirmedStateBeforeIndexAt walks tx diffs of slot",
		testChainReaderPreConfirmedStateBeforeIndexAtTraversesTxs)
	t.Run("PreConfirmedStateBeforeIndexAt rejects index past tx count",
		testChainReaderPreConfirmedStateBeforeIndexAtBadIndex)
	t.Run("PreConfirmedStateBeforeIndexAt rejects block outside chain",
		testChainReaderPreConfirmedStateBeforeIndexAtBlockOutOfRange)
	t.Run("PreConfirmedStateAt resolves base at chain bottom minus one",
		testChainReaderPreConfirmedStateAtBaseAlignsWithBottom)
	t.Run("PreConfirmedStateAt at genesis resolves base via zero hash",
		testChainReaderPreConfirmedStateAtBaseAtGenesis)
	t.Run("PreConfirmedStateAt surfaces bcReader error from base lookup",
		testChainReaderPreConfirmedStateAtBaseError)
	t.Run("TransactionByHash finds tx in any chain entry",
		testChainReaderTransactionByHashAcrossChain)
	t.Run("TransactionByHash returns not-found on miss",
		testChainReaderTransactionByHashMissing)
	t.Run("ReceiptByHash finds receipt and reports owning block",
		testChainReaderReceiptByHashAcrossChain)
	t.Run("ReceiptByHash returns not-found on miss",
		testChainReaderReceiptByHashMissing)
	t.Run("NewChain with single entry produces a length-1 reader",
		testChainReaderNewChainSingleEntry)
	t.Run("NewChain with multiple entries orders newest-first",
		testChainReaderNewChainMultiEntry)
	t.Run("NewChain errors on nil entry or non-contiguous numbers",
		testChainReaderNewChainInvalid)
}

func chainReaderFixture(t *testing.T, count int) *preconfirmed.ChainReader {
	t.Helper()
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= uint64(count); n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}
	v := s.SnapshotForHead(head)
	return &v
}

func testChainReaderNewestFirstOrder(t *testing.T) {
	c := chainReaderFixture(t, 4)
	var got []uint64
	for pc := range c.NewestFirst() {
		got = append(got, pc.Block.Number)
	}
	require.Equal(t, []uint64{4, 3, 2, 1}, got)
}

func testChainReaderOldestFirstOrder(t *testing.T) {
	c := chainReaderFixture(t, 4)
	require.Equal(t, []uint64{1, 2, 3, 4}, chainBlockNumbers(c))
}

func testChainReaderNewestFirstEarlyExit(t *testing.T) {
	c := chainReaderFixture(t, 5)
	var got []uint64
	for pc := range c.NewestFirst() {
		got = append(got, pc.Block.Number)
		if len(got) == 2 {
			break
		}
	}
	require.Equal(t, []uint64{5, 4}, got)
}

func testChainReaderOldestFirstEarlyExit(t *testing.T) {
	c := chainReaderFixture(t, 5)
	var got []uint64
	for pc := range c.OldestFirst() {
		got = append(got, pc.Block.Number)
		if len(got) == 2 {
			break
		}
	}
	require.Equal(t, []uint64{1, 2}, got)
}

func testChainReaderIteratorsAllocFree(t *testing.T) {
	c := chainReaderFixture(t, 5)
	sink := uint64(0)

	newestAllocs := testing.AllocsPerRun(50, func() {
		for pc := range c.NewestFirst() {
			sink += pc.Block.Number
		}
	})
	require.Equal(t, 0.0, newestAllocs, "NewestFirst must be alloc-free")

	oldestAllocs := testing.AllocsPerRun(50, func() {
		for pc := range c.OldestFirst() {
			sink += pc.Block.Number
		}
	})
	require.Equal(t, 0.0, oldestAllocs, "OldestFirst must be alloc-free")
	_ = sink
}

// storageWrite is a single (key, value) write under a fixed contract,
// emitted as exactly one tx. Lets callers interleave shared keys (testing
// last-write-wins / prefix walks) with unshared keys (testing that unrelated
// state from lower slots survives the merge).
type storageWrite struct {
	key   felt.Felt
	value uint64
}

// applyBlockWithStorageWrites applies a block where each write becomes its
// own tx. The block-level merged StateDiff resolves to the last value per
// key (last-write-wins), while the preserved TransactionStateDiffs let
// PreConfirmedStateBeforeIndexAt walk through intermediate values.
func applyBlockWithStorageWrites(
	t *testing.T,
	s *preconfirmed.ChainStorage,
	identifier string,
	contract *felt.Felt,
	writes []storageWrite,
	number uint64,
	head *core.Header,
) {
	t.Helper()
	txCount := len(writes)
	txs := make([]starknet.Transaction, txCount)
	receipts := make([]*starknet.TransactionReceipt, txCount)
	stateDiffs := make([]*starknet.StateDiff, txCount)
	for i, w := range writes {
		hash := new(felt.Felt).SetUint64(number*1000 + uint64(i))
		emptySlice := []*felt.Felt{}
		txs[i] = starknet.Transaction{
			Hash:      hash,
			Type:      starknet.TxnInvoke,
			Version:   &felt.One,
			CallData:  &emptySlice,
			Signature: &emptySlice,
		}
		receipts[i] = &starknet.TransactionReceipt{TransactionHash: hash}
		key := w.key
		value := felt.NewFromUint64[felt.Felt](w.value)
		stateDiffs[i] = &starknet.StateDiff{
			StorageDiffs: map[string][]struct {
				Key   *felt.Felt `json:"key"`
				Value *felt.Felt `json:"value"`
			}{
				contract.String(): {{Key: &key, Value: value}},
			},
		}
	}
	block := starknet.PreConfirmedBlock{
		BlockIdentifier:       identifier,
		Transactions:          txs,
		Receipts:              receipts,
		TransactionStateDiffs: stateDiffs,
		Status:                "PRE_CONFIRMED",
		Timestamp:             uint64(time.Now().Unix()),
		Version:               core.Ver0_14_0.String(),
		SequencerAddress:      feltOne,
		L1GasPrice:            &starknet.GasPrice{PriceInWei: feltOne, PriceInFri: feltOne},
		L2GasPrice:            &starknet.GasPrice{PriceInWei: feltOne, PriceInFri: feltOne},
		L1DAMode:              starknet.Blob,
		L1DataGasPrice:        &starknet.GasPrice{PriceInWei: feltOne, PriceInFri: feltOne},
	}
	_, err := s.ApplyUpdate(block, number, 0, head)
	require.NoError(t, err)
}

func testChainReaderPreConfirmedStateAtComposes(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	contract := felt.FromUint64[felt.Felt](0xC0)
	keyShared := felt.FromUint64[felt.Felt](0x1)
	keyOnlyInSlot1 := felt.FromUint64[felt.Felt](0xA)

	// keyShared is touched in every slot → tests last-write-wins merging.
	// keyOnlyInSlot1 is touched once at the bottom → tests that values from
	// lower slots that no upper slot rewrites are preserved through the merge.
	applyBlockWithStorageWrites(
		t,
		s,
		roundID(1),
		&contract,
		[]storageWrite{{keyShared, 11}, {keyShared, 12}, {keyOnlyInSlot1, 100}},
		1,
		head,
	)
	applyBlockWithStorageWrites(
		t,
		s,
		roundID(2),
		&contract,
		[]storageWrite{{keyShared, 21}, {keyShared, 22}},
		2,
		head,
	)
	applyBlockWithStorageWrites(
		t,
		s,
		roundID(3),
		&contract,
		[]storageWrite{{keyShared, 31}, {keyShared, 32}},
		3,
		head,
	)
	view := s.SnapshotForHead(head)

	cases := []struct {
		blockNumber uint64
		want        uint64
	}{
		{1, 12},
		{2, 22},
		{3, 32},
	}

	ctrl := gomock.NewController(t)
	bc := mocks.NewMockReader(ctrl)
	baseReader := mocks.NewMockStateReader(ctrl)
	bc.EXPECT().StateAtBlockNumber(uint64(0)).
		Return(baseReader, func() error { return nil }, nil).
		Times(len(cases))
	for _, tc := range cases {
		state, closer, err := view.PreConfirmedStateAt(tc.blockNumber, bc)
		require.NoError(t, err)

		gotShared, err := state.ContractStorage(&contract, &keyShared)
		require.NoError(t, err)
		require.Equal(t, felt.FromUint64[felt.Felt](tc.want), gotShared,
			"PreConfirmedStateAt(%d) keyShared should resolve to %d", tc.blockNumber, tc.want)

		// keyOnlyInSlot1 survives every merge — no upper slot rewrites it.
		gotPreserved, err := state.ContractStorage(&contract, &keyOnlyInSlot1)
		require.NoError(t, err)
		require.Equal(t, felt.FromUint64[felt.Felt](100), gotPreserved,
			"PreConfirmedStateAt(%d) keyOnlyInSlot1 must survive the merge", tc.blockNumber)
		require.NoError(t, closer())
	}
}

func testChainReaderPreConfirmedStateAtOutOfRange(t *testing.T) {
	// The two non-empty subtests trip the bounds check before baseState is
	// opened, so passing a nil bcReader is safe and proves the early-return order.
	t.Run("empty chain", func(t *testing.T) {
		// Empty storage yields a zero-value view; callers branch on Length.
		s := preconfirmed.NewChainStorage()
		view := s.SnapshotForHead(headAt(0))
		require.Zero(t, view.Length())
	})

	t.Run("above tip", func(t *testing.T) {
		head := headAt(0)
		s := preconfirmed.NewChainStorage()
		applyBlock(t, s, roundID(1), 0, 1, head)
		view := s.SnapshotForHead(head)
		_, _, err := view.PreConfirmedStateAt(99, nil)
		require.ErrorIs(t, err, pending.ErrPreConfirmedNotFound)
	})

	t.Run("below chain bottom", func(t *testing.T) {
		head := headAt(0)
		s := preconfirmed.NewChainStorage()
		applyBlock(t, s, roundID(1), 0, 1, head)
		applyBlock(t, s, roundID(2), 0, 2, head)
		s.AdvanceTo(headAt(1)) // chain bottom is now slot 2.
		view := s.SnapshotForHead(headAt(1))
		_, _, err := view.PreConfirmedStateAt(1, nil)
		require.ErrorIs(t, err, pending.ErrPreConfirmedNotFound)
	})
}

func testChainReaderPreConfirmedStateBeforeIndexAtTraversesTxs(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	contract := felt.FromUint64[felt.Felt](0xC0)
	key := felt.FromUint64[felt.Felt](0x1)

	// keyShared is touched in every slot — exercises last-write-wins merging
	// across slots AND prefix walks within a slot.
	// keyOnlyInSlot1 is touched once at the bottom — exercises "values from
	// lower slots that no upper slot rewrites survive every merge."
	keyOnlyInSlot1 := felt.FromUint64[felt.Felt](0xA)
	// Slot 1: 2 txs to keyShared (11, 12), 1 tx to keyOnlyInSlot1 (100).
	// Slot 2: 3 txs to keyShared (21, 22, 23) — block diff = 23 for keyShared.
	// Slot 3: 2 txs to keyShared (31, 32)     — block diff = 32 for keyShared.
	applyBlockWithStorageWrites(
		t,
		s,
		roundID(1),
		&contract,
		[]storageWrite{{key, 11}, {key, 12}, {keyOnlyInSlot1, 100}},
		1,
		head,
	)
	applyBlockWithStorageWrites(
		t,
		s,
		roundID(2),
		&contract,
		[]storageWrite{{key, 21}, {key, 22}, {key, 23}},
		2,
		head,
	)
	applyBlockWithStorageWrites(
		t,
		s,
		roundID(3),
		&contract,
		[]storageWrite{{key, 31}, {key, 32}},
		3,
		head,
	)
	view := s.SnapshotForHead(head)

	// Mixed target slots: tip (3) and middle (2). Middle-slot queries verify
	// slot 3 is *excluded* — i.e. the OldestFirst walk breaks at the target
	// rather than merging the full chain.
	cases := []struct {
		blockNumber uint64
		index       uint
		want        uint64
	}{
		// Middle slot — slot 1's full diff is the merge base, slot 3 must not leak in.
		{2, 0, 12}, // no slot-2 txs applied → falls through to slot 1's full diff
		{2, 1, 21}, // slot 2's tx[0]
		{2, 2, 22}, // slot 2's tx[0..1]
		{2, 3, 23}, // slot 2's tx[0..2] — equivalent to PreConfirmedStateAt(2)
		// Tip — slot 1 and slot 2's full diffs are merged first, then slot 3 prefixes.
		{3, 0, 23}, // no slot-3 txs applied → falls through to slot 2's full diff
		{3, 1, 31},
		{3, 2, 32},
	}
	// Chain bottom is slot 1 (head=0); base resolves via StateAtBlockNumber(0)
	// once per case. nil StateReader is fine — every queried key lives in the
	// chain's diff, so pending.State never consults the base.
	ctrl := gomock.NewController(t)
	bc := mocks.NewMockReader(ctrl)
	bc.EXPECT().StateAtBlockNumber(uint64(0)).
		Return(nil, func() error { return nil }, nil).
		Times(len(cases))
	for _, tc := range cases {
		state, closer, err := view.PreConfirmedStateBeforeIndexAt(tc.blockNumber, tc.index, bc)
		require.NoError(t, err)

		gotShared, err := state.ContractStorage(&contract, &key)
		require.NoError(t, err)
		require.Equal(t, felt.FromUint64[felt.Felt](tc.want), gotShared,
			"PreConfirmedStateBeforeIndexAt(%d, %d) keyShared should resolve to %d",
			tc.blockNumber, tc.index, tc.want)

		// keyOnlyInSlot1 must survive every merge — no upper slot rewrites it.
		gotPreserved, err := state.ContractStorage(&contract, &keyOnlyInSlot1)
		require.NoError(t, err)
		require.Equal(t, felt.FromUint64[felt.Felt](100), gotPreserved,
			"PreConfirmedStateBeforeIndexAt(%d, %d) keyOnlyInSlot1 must survive the merge",
			tc.blockNumber, tc.index)
		require.NoError(t, closer())
	}
}

func testChainReaderPreConfirmedStateBeforeIndexAtBadIndex(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	contract := felt.FromUint64[felt.Felt](0xC0)
	key := felt.FromUint64[felt.Felt](0x1)

	applyBlockWithStorageWrites(
		t,
		s,
		roundID(1),
		&contract,
		[]storageWrite{{key, 11}, {key, 12}},
		1,
		head,
	)
	view := s.SnapshotForHead(head)

	// Slot 1 has 2 transactions; index 3 is past the end. The index check
	// runs before baseState, so a nil bcReader is safe here.
	_, _, err := view.PreConfirmedStateBeforeIndexAt(1, 3, nil)
	require.ErrorIs(t, err, pending.ErrTransactionIndexOutOfBounds)
}

func testChainReaderPreConfirmedStateBeforeIndexAtBlockOutOfRange(t *testing.T) {
	// The non-empty subtest trips the bounds check before baseState is opened.
	t.Run("empty chain", func(t *testing.T) {
		// Empty storage yields a zero-value view; callers branch on Length.
		s := preconfirmed.NewChainStorage()
		view := s.SnapshotForHead(headAt(0))
		require.Zero(t, view.Length())
	})

	t.Run("above tip", func(t *testing.T) {
		head := headAt(0)
		s := preconfirmed.NewChainStorage()
		applyBlock(t, s, roundID(1), 0, 1, head)
		view := s.SnapshotForHead(head)
		_, _, err := view.PreConfirmedStateBeforeIndexAt(99, 0, nil)
		require.ErrorIs(t, err, pending.ErrPreConfirmedNotFound)
	})
}

// testChainReaderPreConfirmedStateAtBaseAlignsWithBottom is the regression test
// for the head-vs-snapshot race: even with a 3-entry chain whose canonical
// head sits multiple slots below the tip, the base lookup must hit exactly
// `chain.bottom - 1` and never the live head — otherwise base diffs would
// overlap with chain entries.
func testChainReaderPreConfirmedStateAtBaseAlignsWithBottom(t *testing.T) {
	head := headAt(4)
	s := preconfirmed.NewChainStorage()
	for n := uint64(5); n <= 7; n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}
	view := s.SnapshotForHead(head)
	require.Equal(t, 3, view.Length())

	// bottom = 7 - (3-1) = 5; base must resolve at block 4.
	bc := mocks.NewMockReader(gomock.NewController(t))
	bc.EXPECT().StateAtBlockNumber(uint64(4)).
		Return(nil, func() error { return nil }, nil)

	_, closer, err := view.PreConfirmedStateAt(7, bc)
	require.NoError(t, err)
	require.NoError(t, closer())
}

// testChainReaderPreConfirmedStateAtBaseAtGenesis exercises the bottom==0 branch:
// a single-entry chain at slot 0 has no canonical block below it, so the
// base resolves via the zero hash rather than StateAtBlockNumber.
func testChainReaderPreConfirmedStateAtBaseAtGenesis(t *testing.T) {
	s := preconfirmed.NewChainStorage()
	applyBlock(t, s, roundID(0), 0, 0, nil)
	view := s.SnapshotForHead(nil)

	bc := mocks.NewMockReader(gomock.NewController(t))
	bc.EXPECT().StateAtBlockHash(&felt.Zero).
		Return(nil, func() error { return nil }, nil)

	_, closer, err := view.PreConfirmedStateAt(0, bc)
	require.NoError(t, err)
	require.NoError(t, closer())
}

// testChainReaderPreConfirmedStateAtBaseError verifies that a bcReader failure
// (e.g. base block pruned) is surfaced verbatim — no swallowing, no closer
// returned that the caller might invoke against a half-opened state.
func testChainReaderPreConfirmedStateAtBaseError(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	applyBlock(t, s, roundID(1), 0, 1, head)
	view := s.SnapshotForHead(head)

	wantErr := errors.New("base pruned")
	bc := mocks.NewMockReader(gomock.NewController(t))
	bc.EXPECT().StateAtBlockNumber(uint64(0)).
		Return(nil, nil, wantErr)

	state, closer, err := view.PreConfirmedStateAt(1, bc)
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, state)
	require.Nil(t, closer)
}

// emptyStateDiffPtr returns a fresh empty StateDiff value as a pointer.
func emptyStateDiffPtr() *core.StateDiff {
	sd := core.EmptyStateDiff()
	return &sd
}

// txChainFixture builds a 3-block chain where every transaction is uniquely
// hashed via applyBlockWithStorageWrites's `number*1000 + index` scheme.
// Block 1 carries txs 1000,1001,1002; block 2 carries 2000,2001; block 3 (tip)
// carries 3000,3001,3002.
func txChainFixture(t *testing.T) *preconfirmed.ChainReader {
	t.Helper()
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	contract := felt.FromUint64[felt.Felt](0xC0)
	applyBlockWithStorageWrites(t, s, roundID(1), &contract,
		[]storageWrite{
			{felt.FromUint64[felt.Felt](1), 1},
			{felt.FromUint64[felt.Felt](2), 2},
			{felt.FromUint64[felt.Felt](3), 3},
		},
		1, head)
	applyBlockWithStorageWrites(t, s, roundID(2), &contract,
		[]storageWrite{{felt.FromUint64[felt.Felt](4), 4}, {felt.FromUint64[felt.Felt](5), 5}},
		2, head)
	applyBlockWithStorageWrites(t, s, roundID(3), &contract,
		[]storageWrite{
			{felt.FromUint64[felt.Felt](6), 6},
			{felt.FromUint64[felt.Felt](7), 7},
			{felt.FromUint64[felt.Felt](8), 8},
		},
		3, head)
	v := s.SnapshotForHead(head)
	return &v
}

func testChainReaderTransactionByHashAcrossChain(t *testing.T) {
	c := txChainFixture(t)

	cases := []struct {
		name string
		hash uint64
	}{
		{"bottom block", 1000},
		{"middle block", 2001},
		{"tip block", 3002},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hash := felt.NewFromUint64[felt.Felt](tc.hash)
			tx, err := c.TransactionByHash(hash)
			require.NoError(t, err)
			require.NotNil(t, tx)
			require.True(t, tx.Hash().Equal(hash))
		})
	}
}

func testChainReaderTransactionByHashMissing(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		// Empty storage yields a zero-value view; callers branch on Length.
		s := preconfirmed.NewChainStorage()
		view := s.SnapshotForHead(headAt(0))
		require.Zero(t, view.Length())
	})

	t.Run("unknown hash", func(t *testing.T) {
		c := txChainFixture(t)
		_, err := c.TransactionByHash(felt.NewFromUint64[felt.Felt](999_999))
		require.ErrorIs(t, err, pending.ErrTransactionNotFound)
	})
}

func testChainReaderReceiptByHashAcrossChain(t *testing.T) {
	c := txChainFixture(t)

	cases := []struct {
		name        string
		hash        uint64
		wantBlockNo uint64
	}{
		{"bottom block", 1002, 1},
		{"middle block", 2000, 2},
		{"tip block", 3001, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hash := felt.NewFromUint64[felt.Felt](tc.hash)
			receipt, blockNumber, err := c.ReceiptByHash(hash)
			require.NoError(t, err)
			require.NotNil(t, receipt)
			require.True(t, receipt.TransactionHash.Equal(hash))
			require.Equal(t, tc.wantBlockNo, blockNumber)
		})
	}
}

func testChainReaderReceiptByHashMissing(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		// Empty storage yields a zero-value view; callers branch on Length.
		s := preconfirmed.NewChainStorage()
		view := s.SnapshotForHead(headAt(0))
		require.Zero(t, view.Length())
	})

	t.Run("unknown hash", func(t *testing.T) {
		c := txChainFixture(t)
		_, _, err := c.ReceiptByHash(felt.NewFromUint64[felt.Felt](999_999))
		require.ErrorIs(t, err, pending.ErrTransactionReceiptNotFound)
	})
}

func testChainReaderNewChainSingleEntry(t *testing.T) {
	t.Run("no args produces empty reader", func(t *testing.T) {
		c, err := preconfirmed.NewChain()
		require.NoError(t, err)
		require.Equal(t, 0, c.Length())
		require.Nil(t, c.Head())
	})

	t.Run("non-nil produces length-1 reader pointing at entry", func(t *testing.T) {
		pc := &pending.PreConfirmed{
			Block:       &core.Block{Header: &core.Header{Number: 42}},
			StateUpdate: &core.StateUpdate{StateDiff: emptyStateDiffPtr()},
		}
		c, err := preconfirmed.NewChain(pc)
		require.NoError(t, err)
		require.Equal(t, 1, c.Length())
		require.Same(t, pc, c.Head())
	})
}

func newChainEntry(n uint64) *pending.PreConfirmed {
	return &pending.PreConfirmed{
		Block:       &core.Block{Header: &core.Header{Number: n}},
		StateUpdate: &core.StateUpdate{StateDiff: emptyStateDiffPtr()},
	}
}

func testChainReaderNewChainMultiEntry(t *testing.T) {
	// Entries are given oldest-first; the reader exposes them newest-first with
	// the highest block number as Head.
	c, err := preconfirmed.NewChain(newChainEntry(5), newChainEntry(6), newChainEntry(7))
	require.NoError(t, err)
	require.Equal(t, 3, c.Length())
	require.Equal(t, uint64(7), c.Head().Block.Number)

	var newest []uint64
	for pc := range c.NewestFirst() {
		newest = append(newest, pc.Block.Number)
	}
	require.Equal(t, []uint64{7, 6, 5}, newest)

	require.Equal(t, []uint64{5, 6, 7}, chainBlockNumbers(&c))
}

func testChainReaderNewChainInvalid(t *testing.T) {
	t.Run("nil entry returns error", func(t *testing.T) {
		_, err := preconfirmed.NewChain(newChainEntry(5), nil)
		require.ErrorContains(t, err, "entry 1 is nil")
	})

	t.Run("non-contiguous block numbers return error", func(t *testing.T) {
		_, err := preconfirmed.NewChain(newChainEntry(5), newChainEntry(7))
		require.ErrorContains(t, err, "non-contiguous block numbers at index 1 (7 after 5)")
	})
}

// ---- TestChainStoragePinnedSnapshotImmutability ---------------------------

// TestChainStoragePinnedSnapshotImmutability asserts the core immutability
// invariant of the storage's linked-list design: a snapshot captured at time T
// keeps yielding the same block sequence (and the same per-entry content)
// regardless of any subsequent writer path. Each subtest pins a fresh
// snapshot, exercises one write path against the live storage, and asserts
// the pinned view is unaffected via assertChain (number + identifier + tx
// count, so delta-style content drift is caught alongside structural drift).
func TestChainStoragePinnedSnapshotImmutability(t *testing.T) {
	t.Run("extend", testPinnedSnapshotImmuneToExtend)
	t.Run("replace tip", testPinnedSnapshotImmuneToReplaceTip)
	t.Run("delta at tip", testPinnedSnapshotImmuneToDelta)
	t.Run("advance", testPinnedSnapshotImmuneToAdvance)
}

// pinChain seeds storage with 5 contiguous blocks above head=0 and
// returns (storage, pinnedSnapshot, the seed blocks).
func pinChain(t *testing.T) (
	*preconfirmed.ChainStorage, *preconfirmed.ChainReader, []starknet.PreConfirmedBlock,
) {
	t.Helper()
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	blocks := make([]starknet.PreConfirmedBlock, 5)
	for i := range blocks {
		n := uint64(i + 1)
		blocks[i] = applyBlock(t, s, roundID(n), 0, n, head)
	}
	pinned := s.SnapshotForHead(head)
	return s, &pinned, blocks
}

func testPinnedSnapshotImmuneToExtend(t *testing.T) {
	s, pinned, blocks := pinChain(t)
	head := headAt(0)
	for n := uint64(6); n <= 20; n++ {
		_, err := s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(n), 0), n, 0, head)
		require.NoError(t, err)
	}
	assertChain(t, pinned, rangeEntries(1, blocks)...)
}

func testPinnedSnapshotImmuneToReplaceTip(t *testing.T) {
	s, pinned, blocks := pinChain(t)
	head := headAt(0)
	// Richer-replace must share the existing tip's identifier (roundID(5)).
	for txCount := 1; txCount <= 10; txCount++ {
		_, err := s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(5), txCount), 5, 0, head)
		require.NoError(t, err)
	}
	assertChain(t, pinned, rangeEntries(1, blocks)...)
}

func testPinnedSnapshotImmuneToDelta(t *testing.T) {
	s, pinned, blocks := pinChain(t)
	// Delta merges txs into the tip; baseTxCount must match the slot's current
	// tx count. After each apply the tip's count grows, so the next delta's
	// baseTxCount must follow.
	tipTxCount := uint64(0)
	for _, add := range []int{3, 2, 4} {
		_, err := s.ApplyUpdate(makeTestDelta(roundID(5), add), 5, tipTxCount, headAt(0))
		require.NoError(t, err)
		tipTxCount += uint64(add)
	}
	assertChain(t, pinned, rangeEntries(1, blocks)...)
}

func testPinnedSnapshotImmuneToAdvance(t *testing.T) {
	s, pinned, blocks := pinChain(t)
	// Walk the head forward through every slot — partial trims, then full clear.
	for h := uint64(1); h <= 6; h++ {
		s.AdvanceTo(headAt(h))
	}
	assertChain(t, pinned, rangeEntries(1, blocks)...)
}

// ---- TestChainStorageAllocations ------------------------------------------

// Pre_confirmed RPC handlers walk a snapshot per request, so the storage's
// read fast paths are expected lock-free and allocation-free. The trim/rebuild
// paths do allocate — these tests pin the exact cost (1 ChainReader for a
// view rebuild, 1 ChainReader + keep nodes for AdvanceTo) so a regression
// that walks the whole chain or wraps the reader in a defensive copy shows up.
func TestChainStorageAllocations(t *testing.T) {
	t.Run("SnapshotForHead cached path is alloc-free", testAllocsSnapshotCached)
	t.Run("SnapshotForHead view-trim is alloc-free", testAllocsSnapshotTrim)
	t.Run("AdvanceTo when head hasn't moved is alloc-free", testAllocsAdvanceNoOp)
	t.Run("AdvanceTo trim allocates 1 ChainReader + keep nodes", testAllocsAdvanceTrim)
	t.Run("ApplyUpdate NoChange is alloc-free", testAllocsApplyNoChange)
	t.Run("ApplyUpdate Delta cost is stable", testAllocsApplyDelta)
	t.Run("ApplyUpdate full-block extend cost is stable", testAllocsApplyExtend)
}

// testAllocsSnapshotCached pins the fast path where the reader's head aligns
// with storage's bottom, so the view length equals the stored length and
// SnapshotForHead returns the stored ChainReader without rebuilding.
func testAllocsSnapshotCached(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= 5; n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}

	allocs := testing.AllocsPerRun(100, func() {
		_ = s.SnapshotForHead(head)
	})
	require.Zero(t, allocs)
}

// testAllocsSnapshotTrim pins the view-trim path: the reader's head sits above
// the stored chain's bottom (storage briefly stale before AdvanceTo runs), so
// the view is shorter than the stored chain and can't reuse the stored pointer.
// Value-returning SnapshotForHead constructs the trimmed ChainReader in the
// return slot — no heap allocation.
func testAllocsSnapshotTrim(t *testing.T) {
	storageHead := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= 5; n++ {
		applyBlock(t, s, roundID(n), 0, n, storageHead)
	}
	allocs := testing.AllocsPerRun(100, func() {
		_ = s.SnapshotForHead(headAt(2)) // bottom below head+1 → trimmed view
	})
	require.Zero(t, allocs)
}

func testAllocsAdvanceNoOp(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	for n := uint64(1); n <= 5; n++ {
		applyBlock(t, s, roundID(n), 0, n, head)
	}

	// head still at 0, chain bottom at 1 → wantBottom == bottom → early return,
	// no rebuild. This pins the per-tick cost when the canonical head hasn't
	// moved past the chain bottom (the common steady-state poller tick).
	allocs := testing.AllocsPerRun(100, func() {
		s.AdvanceTo(head)
	})
	require.Zero(t, allocs)
}

// testAllocsAdvanceTrim measures the rebuild cost by subtracting a baseline
// (build-only) from a full run (build + trim). AllocsPerRun amortises a
// deterministic function exactly, so the diff isolates AdvanceTo's
// contribution: 1 ChainReader + `keep` fresh nodes from rebuild().
func testAllocsAdvanceTrim(t *testing.T) {
	head := headAt(0)
	const chainLen, headAfter = 5, 3
	const keep = chainLen - headAfter // blocks headAfter+1 .. chainLen survive
	build := func() *preconfirmed.ChainStorage {
		s := preconfirmed.NewChainStorage()
		for n := uint64(1); n <= chainLen; n++ {
			_, _ = s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(n), 0), n, 0, head)
		}
		return s
	}
	baseline := testing.AllocsPerRun(50, func() { _ = build() })
	withTrim := testing.AllocsPerRun(50, func() {
		s := build()
		s.AdvanceTo(headAt(headAfter))
	})
	require.InDelta(t, float64(keep+1), withTrim-baseline, 0.5)
}

// testAllocsApplyNoChange pins the NoChange short-circuit at the top of
// ApplyUpdate. Every poller tick where the sequencer hasn't moved lands here,
// so a regression that started doing real work on NoChange would be a hot-path
// allocation per tick.
func testAllocsApplyNoChange(t *testing.T) {
	head := headAt(0)
	s := preconfirmed.NewChainStorage()
	applyBlock(t, s, roundID(1), 0, 1, head)
	noChange := starknet.PreConfirmedNoChange{}

	allocs := testing.AllocsPerRun(100, func() {
		_, _ = s.ApplyUpdate(noChange, 1, 0, head)
	})
	require.Zero(t, allocs)
}

// testAllocsApplyDelta and testAllocsApplyExtend pin the apply cost via
// build/with-apply subtraction. The constants below capture the total cost
// (sn2core adapter + storage's own node + ChainReader + escaped pending.PreConfirmed)
// observed on Go 1.24/Opus-test infra; if either changes the test breaks loud
// so the dev makes a conscious bump rather than absorbing a silent regression.
func testAllocsApplyDelta(t *testing.T) {
	head := headAt(0)
	const expectedDeltaCost = 29
	build := func() *preconfirmed.ChainStorage {
		s := preconfirmed.NewChainStorage()
		_, _ = s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(1), 0), 1, 0, head)
		return s
	}
	delta := makeTestDelta(roundID(1), 1)
	baseline := testing.AllocsPerRun(50, func() { _ = build() })
	withApply := testing.AllocsPerRun(50, func() {
		s := build()
		_, _ = s.ApplyUpdate(delta, 1, 0, head)
	})
	require.InDelta(t, float64(expectedDeltaCost), withApply-baseline, 0.5)
}

func testAllocsApplyExtend(t *testing.T) {
	head := headAt(0)
	const expectedExtendCost = 22
	build := func() *preconfirmed.ChainStorage {
		s := preconfirmed.NewChainStorage()
		_, _ = s.ApplyUpdate(makeTestPreConfirmedBlock(roundID(1), 0), 1, 0, head)
		return s
	}
	extendBlock := makeTestPreConfirmedBlock(roundID(2), 0)
	baseline := testing.AllocsPerRun(50, func() { _ = build() })
	withApply := testing.AllocsPerRun(50, func() {
		s := build()
		_, _ = s.ApplyUpdate(extendBlock, 2, 0, head)
	})
	require.InDelta(t, float64(expectedExtendCost), withApply-baseline, 0.5)
}
