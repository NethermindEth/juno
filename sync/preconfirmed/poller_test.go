package preconfirmed_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var feltOne = &felt.One

const tickInterval = 100 * time.Millisecond

// makeTestPreConfirmedBlock returns a starknet.PreConfirmedBlock carrying
// `txCount` synthesised invoke transactions with matching receipts and per-tx
// state diffs.
func makeTestPreConfirmedBlock(identifier string, txCount int) starknet.PreConfirmedBlock {
	txs := make([]starknet.Transaction, txCount)
	receipts := make([]*starknet.TransactionReceipt, txCount)
	stateDiffs := make([]*starknet.StateDiff, txCount)
	for i := range txCount {
		hash := felt.NewFromUint64[felt.Felt](uint64(i + 1))
		emptySlice := []*felt.Felt{}
		txs[i] = starknet.Transaction{
			Hash:      hash,
			Type:      starknet.TxnInvoke,
			Version:   &felt.One,
			CallData:  &emptySlice,
			Signature: &emptySlice,
		}
		receipts[i] = &starknet.TransactionReceipt{TransactionHash: hash}
		stateDiffs[i] = &starknet.StateDiff{
			StorageDiffs: map[string][]struct {
				Key   *felt.Felt `json:"key"`
				Value *felt.Felt `json:"value"`
			}{
				hash.String(): {{
					Key:   felt.NewFromUint64[felt.Felt](uint64(i + 1)),
					Value: felt.NewFromUint64[felt.Felt](uint64(i + 1)),
				}},
			},
		}
	}
	return starknet.PreConfirmedBlock{
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
}

// makeTestDelta returns a PreConfirmedDeltaUpdate that appends `addedCount`
// transactions/receipts/state diffs under the given block identifier.
func makeTestDelta(identifier string, addedCount int) starknet.PreConfirmedDeltaUpdate {
	txs := make([]starknet.Transaction, addedCount)
	receipts := make([]*starknet.TransactionReceipt, addedCount)
	stateDiffs := make([]*starknet.StateDiff, addedCount)
	for i := range addedCount {
		hash := new(felt.Felt).SetUint64(uint64(100 + i))
		emptySlice := []*felt.Felt{}
		txs[i] = starknet.Transaction{
			Hash:      hash,
			Type:      starknet.TxnInvoke,
			Version:   new(felt.Felt).SetUint64(1),
			CallData:  &emptySlice,
			Signature: &emptySlice,
		}
		receipts[i] = &starknet.TransactionReceipt{TransactionHash: hash}
		stateDiffs[i] = &starknet.StateDiff{}
	}
	return starknet.PreConfirmedDeltaUpdate{
		BlockIdentifier:       identifier,
		Transactions:          txs,
		Receipts:              receipts,
		TransactionStateDiffs: stateDiffs,
	}
}

type chainFixture struct {
	bc   *blockchain.Blockchain
	db   db.KeyValueStore
	head *core.Header
}

// newChainFixture seeds the underlying db with a synthetic header at Number=0
// and chain height = 0.
func newChainFixture(t *testing.T) *chainFixture {
	t.Helper()
	testDB := memory.New()
	bc := blockchain.New(testDB, &networks.Sepolia)

	header := &core.Header{
		Number:     0,
		Hash:       new(felt.Felt).SetUint64(1),
		ParentHash: &felt.Zero,
	}
	require.NoError(t, core.WriteBlockHeaderByNumber(testDB, header))
	require.NoError(t, core.WriteChainHeight(testDB, 0))

	return &chainFixture{bc: bc, db: testDB, head: header}
}

// advanceHead writes a synthetic header at head.Number+1 to the db and bumps
// chain height. Returns the new head.
func (f *chainFixture) advanceHead(t *testing.T) *core.Header {
	t.Helper()
	next := &core.Header{
		Number:     f.head.Number + 1,
		Hash:       new(felt.Felt).SetUint64(f.head.Number + 2),
		ParentHash: f.head.Hash,
	}
	require.NoError(t, core.WriteBlockHeaderByNumber(f.db, next))
	require.NoError(t, core.WriteChainHeight(f.db, next.Number))
	f.head = next
	return next
}

type harness struct {
	poller  *preconfirmed.Poller
	storage *preconfirmed.ChainStorage
	head    *core.Header
	highest *atomic.Pointer[core.Header]
	sub     *feed.Subscription[*pending.PreConfirmed]
}

// wirePoller creates the in-memory bits (storage, feed, atomic header, poller)
// against an already-constructed blockchain and DataSource.
func wirePoller(
	t *testing.T,
	bc *blockchain.Blockchain,
	head *core.Header,
	ds preconfirmed.DataSource,
) harness {
	t.Helper()
	storage := preconfirmed.NewChainStorage()
	out := feed.New[*pending.PreConfirmed]()
	sub := out.SubscribeKeepLast()
	t.Cleanup(sub.Unsubscribe)

	highest := &atomic.Pointer[core.Header]{}
	highest.Store(head)

	p := preconfirmed.NewPoller(ds, storage, bc, out, highest, tickInterval, log.NewNopZapLogger())
	return harness{
		poller:  p,
		storage: storage,
		head:    head,
		highest: highest,
		sub:     sub,
	}
}

// expectedEntry describes one slot of an expected chain snapshot. Used by
// assertChain to pin down the full storage state after a tick.
type expectedEntry struct {
	number     uint64
	identifier string
	txCount    int
}

// entry derives an expectedEntry from the wire-side block whose application
// produced the stored slot, plus any deltas merged on top of it. Identifier
// is taken from the seed (deltas preserve identifier); tx count is the sum.
// Pointer args avoid copying the ~168-byte PreConfirmedBlock value.
func entry(
	number uint64,
	block *starknet.PreConfirmedBlock,
	deltas ...*starknet.PreConfirmedDeltaUpdate,
) expectedEntry {
	txCount := len(block.Transactions)
	for _, d := range deltas {
		txCount += len(d.Transactions)
	}
	return expectedEntry{number, block.BlockIdentifier, txCount}
}

// assertChain pins down a snapshot's full contents in oldest-first order.
// Catches off-by-one length mistakes, identifier preservation/replacement,
// and tx-count merges that a head-only check would silently miss.
func assertChain(t *testing.T, snap *preconfirmed.ChainReader, want ...expectedEntry) {
	t.Helper()
	require.NotNil(t, snap, "snapshot must be non-nil")
	require.Equal(t, len(want), snap.Length(), "chain length")
	i := 0
	for pc := range snap.OldestFirst() {
		require.Equal(t, want[i].number, pc.Block.Number, "entry %d block number", i)
		require.Equal(t, want[i].identifier, pc.BlockIdentifier, "entry %d identifier", i)
		require.Equal(t, want[i].txCount, len(pc.Block.Transactions), "entry %d tx count", i)
		i++
	}
}

// Empty storage, sequencer at head+1: tick polls latest once and bootstraps
// the chain — no backfill needed.
func TestPollerColdBootstrapNoGap(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r0", 1)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
		Return(block1, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()
		time.Sleep(tickInterval)
		synctest.Wait()

		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view, entry(1, &block1))
	})
}

// Empty storage, sequencer several blocks ahead of head: backfill walks the
// intermediate heights with blank hints before applying the latest.
func TestPollerColdBootstrapWithGap(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r1", 0)
	block2 := makeTestPreConfirmedBlock("r2", 0)
	block3 := makeTestPreConfirmedBlock("r3", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		ds.EXPECT().
			PreConfirmedBlockLatest(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(block3, uint64(3), nil),
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
			Return(block1, nil),
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "", uint64(0)).
			Return(block2, nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view,
			entry(1, &block1),
			entry(2, &block2),
			entry(3, &block3),
		)
	})
}

// mostRecent already at target height and server returns NoChange: tick is a
// pure no-op — no backfill, no apply, storage pointer unchanged.
func TestPollerSameHeightNoBackfill(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
		Return(starknet.PreConfirmedNoChange{}, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed, h.head.Number+1, 0, h.head)
		require.NoError(t, err)
		before := h.storage.SnapshotForHead(h.head)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		after := h.storage.SnapshotForHead(h.head)
		require.Same(t, before.Head(), after.Head(),
			"NoChange must leave the chain pointer unchanged")
		assertChain(t, &after, entry(1, &seed))
	})
}

// mostRecent at target height and server returns a Delta enriching that
// block: no backfill, apply merges the delta into the same slot, the stored
// chain still has length 1 but with the appended transactions visible.
func TestPollerSameHeightDeltaAppliesToSlot(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)
	delta := makeTestDelta("r0", 2)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// Delta hint matches mostRecent: identifier="r0", txCount=0.
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
		Return(delta, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed, h.head.Number+1, 0, h.head)
		require.NoError(t, err)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		// Delta preserves seed.identifier and appends its own txs to seed's.
		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view, entry(1, &seed, &delta))
	})
}

// Sequencer advanced by exactly one: backfill re-polls mostRecent's own
// height with its identifier+txCount hints, and the server replies with a
// Delta carrying the final txs the sequencer appended before publishing the
// next block. The delta merges into the existing slot (identifier preserved,
// txs summed), then the new most recent is applied.
func TestPollerForwardJumpFinalisesMostRecent(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)
	finaliseDelta := makeTestDelta("r0", 3) // 3 final txs appended at block 1
	block2 := makeTestPreConfirmedBlock("r1", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		ds.EXPECT().
			PreConfirmedBlockLatest(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(block2, uint64(2), nil),
		// Finalise poll: server replies with a Delta keyed off the stored
		// identifier+txCount; carries any txs appended since.
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r0", uint64(0)).
			Return(finaliseDelta, nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed, h.head.Number+1, 0, h.head)
		require.NoError(t, err)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view,
			entry(1, &seed, &finaliseDelta), // seed + delta merged at block 1
			entry(2, &block2),
		)
	})
}

// Sequencer jumped multiple blocks ahead: backfill finalises mostRecent and
// then walks every intermediate height with blank hints before applying the
// new most recent.
func TestPollerLargeJumpWalksGap(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)
	finaliseReply := makeTestPreConfirmedBlock("r0", 0) // same content → slot preserved
	block2 := makeTestPreConfirmedBlock("r2", 0)
	block3 := makeTestPreConfirmedBlock("r3", 0)
	block4 := makeTestPreConfirmedBlock("r4", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		ds.EXPECT().
			PreConfirmedBlockLatest(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(block4, uint64(4), nil),
		// Finalise mostRecent (1) with delta hints.
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r0", uint64(0)).
			Return(finaliseReply, nil),
		// Walk intermediate blocks 2, 3 with blank hints.
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "", uint64(0)).
			Return(block2, nil),
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(3), "", uint64(0)).
			Return(block3, nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed, h.head.Number+1, 0, h.head)
		require.NoError(t, err)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view,
			entry(1, &seed), // identifier preserved by finalise
			entry(2, &block2),
			entry(3, &block3),
			entry(4, &block4),
		)
	})
}

// highestBlockHeader sits above head (canonical sync is still catching up):
// the atTip gate short-circuits the tick before any wire call.
func TestPollerNotAtTipSkipsAllWork(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// No expectations set: any wire call fails the test.

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		h.highest.Store(&core.Header{Number: h.head.Number + 5}) // not at tip

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.storage.SnapshotForHead(h.head)
		require.Zero(t, view.Length())
	})
}

// PreConfirmedBlockLatest errors: tick aborts immediately, no backfill or apply,
// storage stays empty for the next tick to retry from scratch.
func TestPollerLatestErrorSkipsApply(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, uint64(0), errors.New("wire boom"))

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		view := h.storage.SnapshotForHead(h.head)
		require.Zero(t, view.Length(), "latest error must not produce any storage state")
	})
}

// backfill's per-block poll errors mid-gap: tick aborts before the final apply
// at target, so storage remains empty (next tick reconciles).
func TestPollerBackfillErrorSkipsApply(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	latestReply := makeTestPreConfirmedBlock("r3", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		ds.EXPECT().
			PreConfirmedBlockLatest(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(latestReply, uint64(3), nil),
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("backfill boom")),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		view := h.storage.SnapshotForHead(h.head)
		require.Zero(t, view.Length(), "tick aborts before any apply when backfill errors")
	})
}

// Across three consecutive ticks the sequencer advances one block at a time:
// each tick finalises the prior mostRecent and lands a new most recent, so the
// chain grows monotonically from 1 to 3 entries.
func TestPollerMultiTickExtendsChain(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r1", 0)
	block2 := makeTestPreConfirmedBlock("r2", 0)
	block3 := makeTestPreConfirmedBlock("r3", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// Script every wire call across the three ticks in chronological order.
	// Finalise polls return the same block already at that slot so
	// shouldPreserveSlot keeps the entry instead of replacing it.
	gomock.InOrder(
		// Tick 1: cold bootstrap.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(block1, uint64(1), nil),
		// Tick 2: sequencer advanced; latest + finalise of block 1.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r1", uint64(0)).
			Return(block2, uint64(2), nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r1", uint64(0)).
			Return(block1, nil),
		// Tick 3: advanced again; latest + finalise of block 2.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r2", uint64(0)).
			Return(block3, uint64(3), nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "r2", uint64(0)).
			Return(block2, nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view1 := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view1, entry(1, &block1))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()
		view2 := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view2,
			entry(1, &block1),
			entry(2, &block2),
		)

		// Tick 3.
		time.Sleep(tickInterval)
		synctest.Wait()
		view3 := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view3,
			entry(1, &block1),
			entry(2, &block2),
			entry(3, &block3),
		)
	})
}

// Canonical head advances past one of the stored pre_confirmed entries:
// AdvanceTo at the top of tick drops the now-committed entry from the chain,
// leaving only entries that are still above the new head.
func TestPollerHeadAdvancesDropsCommittedEntries(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed1 := makeTestPreConfirmedBlock("r1", 0) // will be dropped after head advances past it
	seed2 := makeTestPreConfirmedBlock("r2", 0)

	var latestBlockNumber atomic.Uint64
	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, string, uint64) (starknet.PreConfirmedUpdate, uint64, error) {
			return starknet.PreConfirmedNoChange{}, latestBlockNumber.Load(), nil
		})

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed1, h.head.Number+1, 0, h.head)
		require.NoError(t, err)
		_, err = h.storage.ApplyUpdate(seed2, h.head.Number+2, 0, h.head)
		require.NoError(t, err)
		before := h.storage.SnapshotForHead(h.head)
		require.Equal(t, 2, before.Length())

		// Canonical head advances by one — block head+1 is now committed.
		newHead := fx.advanceHead(t)
		h.highest.Store(newHead)
		latestBlockNumber.Store(newHead.Number + 1)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		// seed1 committed → dropped; only seed2 remains at the new head+1.
		view := h.storage.SnapshotForHead(newHead)
		assertChain(t, &view, entry(newHead.Number+1, &seed2))
	})
}

// Sequencer rewinds: PreConfirmedBlockLatest reports a height BELOW the chain's
// current most recent with a new identifier (a new round started at a lower
// slot). Tick must not backfill (target < fromBlock); apply hits the in-chain
// replace path which truncates everything above the replaced slot and
// installs the new block there.
func TestPollerReorgLowerHeightDifferentIdentifier(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed1 := makeTestPreConfirmedBlock("r1", 0)
	seed2 := makeTestPreConfirmedBlock("r2", 0)
	seed3 := makeTestPreConfirmedBlock("r3", 0)
	// New round arrives at block 2 with a different identifier — anything
	// above (block 3) must be dropped.
	replacement := makeTestPreConfirmedBlock("rZ", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// Hint matches the most recent (block 3, "r3", 0 txs) the poller would carry.
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r3", uint64(0)).
		Return(replacement, uint64(2), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed1, h.head.Number+1, 0, h.head)
		require.NoError(t, err)
		_, err = h.storage.ApplyUpdate(seed2, h.head.Number+2, 0, h.head)
		require.NoError(t, err)
		_, err = h.storage.ApplyUpdate(seed3, h.head.Number+3, 0, h.head)
		require.NoError(t, err)
		before := h.storage.SnapshotForHead(h.head)
		require.Equal(t, 3, before.Length())

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		// Block 2 swapped to the new identifier; block 3 truncated.
		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view,
			entry(1, &seed1),
			entry(2, &replacement),
		)
	})
}

func TestPollerReorgSameHeightDifferentIdentifier(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed1 := makeTestPreConfirmedBlock("r1", 0)
	seed2 := makeTestPreConfirmedBlock("r2", 0)
	seed3 := makeTestPreConfirmedBlock("r3", 0)
	// New round at the same slot — different identifier replaces the most recent.
	replacement := makeTestPreConfirmedBlock("rZ", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r3", uint64(0)).
		Return(replacement, uint64(3), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		_, err := h.storage.ApplyUpdate(seed1, h.head.Number+1, 0, h.head)
		require.NoError(t, err)
		_, err = h.storage.ApplyUpdate(seed2, h.head.Number+2, 0, h.head)
		require.NoError(t, err)
		_, err = h.storage.ApplyUpdate(seed3, h.head.Number+3, 0, h.head)
		require.NoError(t, err)
		before := h.storage.SnapshotForHead(h.head)
		require.Equal(t, 3, before.Length())

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		// Deepest slot replaced; lower entries and length untouched.
		view := h.storage.SnapshotForHead(h.head)
		assertChain(t, &view,
			entry(1, &seed1),
			entry(2, &seed2),
			entry(3, &replacement),
		)
	})
}

// Successful apply must publish the affected entry on the broadcast.
func TestPollerBroadcastsOnApply(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r0", 1)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
		Return(block1, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		select {
		case pc := <-h.sub.Recv():
			require.NotNil(t, pc)
			require.Equal(t, uint64(1), pc.Block.Number)
			require.Equal(t, "r0", pc.BlockIdentifier)
		default:
			t.Fatal("expected a pre_confirmed broadcast on successful apply")
		}
	})
}

// NoChange must not publish anything
func TestPollerSilentOnNoChange(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
		Return(starknet.PreConfirmedNoChange{}, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		// Seed directly through storage; this does NOT publish (only the
		// poller's apply wrapper does Send), so the feed starts clean.
		_, err := h.storage.ApplyUpdate(seed, h.head.Number+1, 0, h.head)
		require.NoError(t, err)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		select {
		case pc := <-h.sub.Recv():
			t.Fatalf("did not expect a broadcast on NoChange, got %+v", pc)
		default:
		}
	})
}
