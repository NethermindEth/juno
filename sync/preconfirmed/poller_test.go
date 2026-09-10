package preconfirmed_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
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
		emptySlice := []felt.Felt{}
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
		emptySlice := []felt.Felt{}
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

// harness is a poller wired against a blockchain and a DataSource, plus the
// highest-known header its atTip gate reads.
type harness struct {
	poller  *preconfirmed.Poller
	highest *atomic.Pointer[core.Header]
}

// wirePoller builds a poller against an already-constructed blockchain and
// DataSource. The highest-known header starts at head, so the poller is at tip
// unless a test moves it.
func wirePoller(
	t *testing.T,
	bc *blockchain.Blockchain,
	head *core.Header,
	ds preconfirmed.DataSource,
) harness {
	t.Helper()
	highest := &atomic.Pointer[core.Header]{}
	highest.Store(head)

	p := preconfirmed.NewPoller(ds, bc, highest, tickInterval, log.NewNopZapLogger())
	return harness{poller: p, highest: highest}
}

// chain reads the poller's current pre-confirmed view the way any consumer does.
func (h harness) chain(t *testing.T) preconfirmed.ChainReader {
	t.Helper()
	view, err := h.poller.PreConfirmedChain()
	require.NoError(t, err)
	return view
}

// expectedEntry describes one slot of an expected chain view. Used by
// assertChain to pin down the full chain after a tick.
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

// blankEntry describes the placeholder PreConfirmedChain returns while nothing
// has been polled for the slot above the canonical head: an empty block carrying
// the blank identifier.
//
//nolint:unparam // number is 1 all the time by chance
func blankEntry(number uint64) expectedEntry {
	return expectedEntry{number: number, identifier: feeder.PreConfirmedBlankIdentifier, txCount: 0}
}

// assertChain pins down a chain view's full contents in oldest-first order.
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

// Nothing polled yet, sequencer at head+1: tick polls latest once and bootstraps
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

		view := h.chain(t)
		assertChain(t, &view, entry(1, &block1))
	})
}

// Nothing polled yet, sequencer several blocks ahead of head: backfill walks the
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

		view := h.chain(t)
		assertChain(t, &view,
			entry(1, &block1),
			entry(2, &block2),
			entry(3, &block3),
		)
	})
}

// Empty blockchain (pre-genesis): Run stalls without touching the data source,
// and there is no pre-confirmed chain to read. Once the genesis block lands, the
// next tick bootstraps the chain as usual.
func TestPollerStallsUntilGenesis(t *testing.T) {
	t.Parallel()

	// No header and no chain height written: Height() fails with ErrKeyNotFound.
	testDB := memory.New()
	bc := blockchain.New(testDB, &networks.Sepolia)
	genesis := &core.Header{
		Number:     0,
		Hash:       new(felt.Felt).SetUint64(1),
		ParentHash: &felt.Zero,
	}

	block1 := makeTestPreConfirmedBlock("r0", 1)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// Registered before the bubble starts: gomock expectations must not be added
	// while the poller goroutine is live (Return writes the call unlocked, which
	// the race detector flags). Times(1) by default, so a poll before genesis
	// would consume it and fail the post-genesis tick with an unexpected call.
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
		Return(block1, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, bc, genesis, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Pre-genesis ticks: the poller stalls on its height guard. Reading the
		// chain fails the same way that guard does.
		time.Sleep(3 * tickInterval)
		synctest.Wait()
		_, err := h.poller.PreConfirmedChain()
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		// Genesis lands.
		require.NoError(t, core.WriteBlockHeaderByNumber(testDB, genesis))
		require.NoError(t, core.WriteChainHeight(testDB, 0))

		// One interval for the guard to observe the new head, one more for the
		// first real polling tick.
		time.Sleep(2 * tickInterval)
		synctest.Wait()

		view := h.chain(t)
		assertChain(t, &view, entry(1, &block1))
	})
}

// The chain tip is already at the server's height and the server returns
// NoChange: the tick is a pure no-op — no backfill poll, and the chain reads
// back unchanged.
func TestPollerSameHeightNoBackfill(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		// Tick 1: seed the chain through the wire.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed, uint64(1), nil),
		// Tick 2: same height, nothing new. No PreConfirmedBlockByNumber is
		// expected, so a backfill poll would fail the test.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
			Return(starknet.PreConfirmedNoChange{}, uint64(1), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()
		view = h.chain(t)
		assertChain(t, &view, entry(1, &seed))
	})
}

// The chain tip is at the server's height and the server returns a Delta
// enriching that block: no backfill, the delta merges into the same slot, the
// chain still has length 1 but with the appended transactions visible.
func TestPollerSameHeightDeltaAppliesToSlot(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)
	delta := makeTestDelta("r0", 2)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		// Tick 1: seed the chain through the wire.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed, uint64(1), nil),
		// Tick 2: delta hint matches the seeded tip: identifier="r0", txCount=0.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
			Return(delta, uint64(1), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed))

		// Tick 2: delta preserves seed.identifier and appends its own txs to seed's.
		time.Sleep(tickInterval)
		synctest.Wait()
		view = h.chain(t)
		assertChain(t, &view, entry(1, &seed, &delta))
	})
}

// Sequencer advanced by exactly one: backfill re-polls the tip's own height with its
// identifier+txCount hints, and the server replies with a Delta carrying the final txs
// appended before the next block. The delta merges into the existing slot (identifier
// preserved, txs summed), then the new most recent is applied.
func TestPollerForwardJumpFinalisesMostRecent(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)
	finaliseDelta := makeTestDelta("r0", 3) // 3 final txs appended at block 1
	block2 := makeTestPreConfirmedBlock("r1", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		// Tick 1: seed block 1.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed, uint64(1), nil),
		// Tick 2: the sequencer moved on to block 2.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
			Return(block2, uint64(2), nil),
		// Finalise poll: server replies with a Delta keyed off the stored
		// identifier+txCount; carries any txs appended since.
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r0", uint64(0)).
			Return(finaliseDelta, nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()
		view = h.chain(t)
		assertChain(t, &view,
			entry(1, &seed, &finaliseDelta), // seed + delta merged at block 1
			entry(2, &block2),
		)
	})
}

// Sequencer jumped multiple blocks ahead: backfill finalises the old tip and
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
		// Tick 1: seed block 1.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed, uint64(1), nil),
		// Tick 2: the sequencer is at block 4.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
			Return(block4, uint64(4), nil),
		// Finalise the old tip (1) with delta hints.
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r0", uint64(0)).
			Return(finaliseReply, nil),
		// Walk intermediate blocks 2, 3 with blank hints.
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "", uint64(0)).
			Return(block2, nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(3), "", uint64(0)).
			Return(block3, nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()
		view = h.chain(t)
		assertChain(t, &view,
			entry(1, &seed), // identifier preserved by finalise
			entry(2, &block2),
			entry(3, &block3),
			entry(4, &block4),
		)
	})
}

// highestBlockHeader sits above head (canonical sync is still catching up):
// the atTip gate short-circuits the tick before any wire call, and the chain
// stays at the blank placeholder.
func TestPollerNotAtTipSkipsAllWork(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// No expectations set: any wire call fails the test.

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		h.highest.Store(&core.Header{Number: fx.head.Number + 5}) // not at tip

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, blankEntry(1))
	})
}

// PreConfirmedBlockLatest errors: tick aborts immediately, no backfill or apply;
// nothing is stored, so the next tick retries from scratch.
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

		// A latest error must not produce any chain state.
		view := h.chain(t)
		assertChain(t, &view, blankEntry(1))
	})
}

// backfill's per-block poll errors mid-gap: tick aborts before the final apply
// at target, so nothing is stored (next tick reconciles).
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

		// The tick aborts before any apply when backfill errors.
		view := h.chain(t)
		assertChain(t, &view, blankEntry(1))
	})
}

// The gateway answering 400 to a pre-confirmed poll surfaces as
// feeder.ErrPreConfirmedBlockNotFound; the tick treats it as "the window is
// empty — nothing to do": it stores nothing, and the next tick picks up the
// window normally once it opens.
func TestPollerLatestNotFoundSkipsTickAndRecovers(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r1", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(nil, uint64(0), fmt.Errorf("querying: %w", feeder.ErrPreConfirmedBlockNotFound)),
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(block1, uint64(1), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1: nothing in the pre-confirmed window — the tick is a no-op.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, blankEntry(1))

		// Tick 2: the window opened; polling proceeds normally.
		time.Sleep(tickInterval)
		synctest.Wait()
		view = h.chain(t)
		assertChain(t, &view, entry(1, &block1))
	})
}

// A block that left the pre-confirmed window mid-backfill surfaces as
// feeder.ErrPreConfirmedBlockNotFound through backfill's wrapping; the tick
// abandons the rest of the fill — nothing is applied, not even the already
// fetched latest — and the next tick reconciles the whole gap from scratch.
func TestPollerBackfillNotFoundSkipsApplyAndRecovers(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r1", 0)
	block2 := makeTestPreConfirmedBlock("r2", 0)
	latestReply := makeTestPreConfirmedBlock("r3", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	// Exactly-once, in-order expectations double as a behavioral pin: a tick
	// that kept backfilling past the not-found would consume tick 2's
	// expectations early and fail the script.
	gomock.InOrder(
		// Tick 1: latest says 3, but block 1 already left the window.
		ds.EXPECT().
			PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(latestReply, uint64(3), nil),
		ds.EXPECT().
			PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
			Return(nil, fmt.Errorf(
				"polling pre-confirmed for number 1: %w", feeder.ErrPreConfirmedBlockNotFound,
			)),
		// Tick 2: nothing was stored, so the poller re-walks the full gap.
		ds.EXPECT().
			PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(latestReply, uint64(3), nil),
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

		// Tick 1: the mid-fill not-found aborts the tick before any apply.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, blankEntry(1))

		// Tick 2: the full gap backfills and the latest lands on top.
		time.Sleep(tickInterval)
		synctest.Wait()
		view = h.chain(t)
		assertChain(t, &view,
			entry(1, &block1),
			entry(2, &block2),
			entry(3, &latestReply),
		)
	})
}

// Across three consecutive ticks the sequencer advances one block at a time:
// each tick finalises the prior tip and lands a new most recent, so the chain
// grows monotonically from 1 to 3 entries.
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
		view1 := h.chain(t)
		assertChain(t, &view1, entry(1, &block1))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()
		view2 := h.chain(t)
		assertChain(t, &view2,
			entry(1, &block1),
			entry(2, &block2),
		)

		// Tick 3.
		time.Sleep(tickInterval)
		synctest.Wait()
		view3 := h.chain(t)
		assertChain(t, &view3,
			entry(1, &block1),
			entry(2, &block2),
			entry(3, &block3),
		)
	})
}

// Canonical head advances past one of the stored pre_confirmed entries: the
// next tick realigns the chain to the new head, dropping the now-committed
// entry and leaving only entries that are still above it.
func TestPollerHeadAdvancesDropsCommittedEntries(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed1 := makeTestPreConfirmedBlock("r1", 0) // dropped once head advances past it
	seed2 := makeTestPreConfirmedBlock("r2", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		// Tick 1: seed blocks 1 and 2 (latest at 2, backfill walks 1).
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed2, uint64(2), nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
			Return(seed1, nil),
		// Tick 2: head is now 1; the surviving tip (block 2, "r2") is re-polled
		// unchanged.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r2", uint64(0)).
			Return(starknet.PreConfirmedNoChange{}, uint64(2), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed1), entry(2, &seed2))

		// Canonical head advances by one — block 1 is now committed.
		newHead := fx.advanceHead(t)
		h.highest.Store(newHead)

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()

		// seed1 committed → dropped; only seed2 remains at the new head+1.
		view = h.chain(t)
		assertChain(t, &view, entry(newHead.Number+1, &seed2))
	})
}

// Sequencer rewinds: PreConfirmedBlockLatest reports a height BELOW the chain's
// current tip with a new identifier (a new round started at a lower slot). Tick
// must not backfill (target < fromBlock); apply hits the in-chain replace path
// which truncates everything above the replaced slot and installs the new block
// there.
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
	gomock.InOrder(
		// Tick 1: seed blocks 1..3 (latest at 3, backfill walks 1 and 2).
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed3, uint64(3), nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
			Return(seed1, nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "", uint64(0)).
			Return(seed2, nil),
		// Tick 2: hint carries the tip (block 3, "r3", 0 txs); the server
		// answers with a new round at block 2.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r3", uint64(0)).
			Return(replacement, uint64(2), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed1), entry(2, &seed2), entry(3, &seed3))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()

		// Block 2 swapped to the new identifier; block 3 truncated.
		view = h.chain(t)
		assertChain(t, &view,
			entry(1, &seed1),
			entry(2, &replacement),
		)
	})
}

// A new round starts at the tip's own height: the tip is replaced in place and
// the lower entries are untouched.
func TestPollerReorgSameHeightDifferentIdentifier(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed1 := makeTestPreConfirmedBlock("r1", 0)
	seed2 := makeTestPreConfirmedBlock("r2", 0)
	seed3 := makeTestPreConfirmedBlock("r3", 0)
	// New round at the same slot — different identifier replaces the tip.
	replacement := makeTestPreConfirmedBlock("rZ", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		// Tick 1: seed blocks 1..3 (latest at 3, backfill walks 1 and 2).
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed3, uint64(3), nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
			Return(seed1, nil),
		ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "", uint64(0)).
			Return(seed2, nil),
		// Tick 2: same height as the tip, different identifier.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r3", uint64(0)).
			Return(replacement, uint64(3), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1.
		time.Sleep(tickInterval)
		synctest.Wait()
		view := h.chain(t)
		assertChain(t, &view, entry(1, &seed1), entry(2, &seed2), entry(3, &seed3))

		// Tick 2.
		time.Sleep(tickInterval)
		synctest.Wait()

		// Deepest slot replaced; lower entries and length untouched.
		view = h.chain(t)
		assertChain(t, &view,
			entry(1, &seed1),
			entry(2, &seed2),
			entry(3, &replacement),
		)
	})
}

// classesAt returns the NewClasses registered on the view's slot at blockNumber.
func classesAt(
	snap *preconfirmed.ChainReader,
	blockNumber uint64,
) map[felt.Felt]core.ClassDefinition {
	for pc := range snap.OldestFirst() {
		if pc.Block.Number == blockNumber {
			return pc.NewClasses
		}
	}
	return nil
}

func TestPollerBackfillFetchesDeclaredClasses(t *testing.T) {
	t.Parallel()

	// A full-block re-poll (a new round at the old tip) fetches that block's own declared
	// classes.
	t.Run("full block re-poll fetches its declared classes", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		classHash := new(felt.Felt).SetUint64(0xC1A55)
		classDef := &core.SierraClass{}

		seed := makeTestPreConfirmedBlock("r1", 0) // stored old tip, declares nothing
		repoll := makeTestPreConfirmedBlock("r1b", 1)
		repoll.TransactionStateDiffs[0].DeclaredClasses = []struct {
			ClassHash         *felt.Felt `json:"class_hash"`
			CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
		}{{ClassHash: classHash}}
		newTip := makeTestPreConfirmedBlock("r2", 0)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockStarknetData(ctrl)
		gomock.InOrder(
			// Tick 1: seed the old tip.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(seed, uint64(1), nil),
			// Tick 2: forward jump; the old tip is re-polled as a new round.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r1", uint64(0)).
				Return(newTip, uint64(2), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r1", uint64(0)).
				Return(repoll, nil),
		)
		ds.EXPECT().Class(gomock.Any(), classHash).Return(classDef, nil)

		synctest.Test(t, func(t *testing.T) {
			h := wirePoller(t, fx.bc, fx.head, ds)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// Tick 1.
			time.Sleep(tickInterval)
			synctest.Wait()
			// Tick 2.
			time.Sleep(tickInterval)
			synctest.Wait()

			view := h.chain(t)
			require.Equal(t, classDef, classesAt(&view, 1)[*classHash],
				"the re-polled full block's declared class must be fetched and land on slot 1")
		})
	})

	// A delta re-poll fetches both the stored tip's already-declared classes and the
	// delta's newly-declared ones.
	t.Run("delta re-poll fetches stored tip and delta classes", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		storedClass := new(felt.Felt).SetUint64(0xADD)
		deltaClass := new(felt.Felt).SetUint64(0xC1A55)
		storedDef := &core.SierraClass{}
		deltaDef := &core.SierraClass{}

		seed := makeTestPreConfirmedBlock("r1", 1) // stored old tip declares storedClass
		seed.TransactionStateDiffs[0].DeclaredClasses = []struct {
			ClassHash         *felt.Felt `json:"class_hash"`
			CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
		}{{ClassHash: storedClass}}
		repoll := makeTestDelta("r1", 1) // same-round delta declares deltaClass
		repoll.TransactionStateDiffs[0].DeclaredClasses = []struct {
			ClassHash         *felt.Felt `json:"class_hash"`
			CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
		}{{ClassHash: deltaClass}}
		newTip := makeTestPreConfirmedBlock("r2", 0)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockStarknetData(ctrl)
		gomock.InOrder(
			// Tick 1: seed the old tip. The tick's apply of the latest does not
			// fetch classes, so storedClass is only resolved by the re-poll below.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(seed, uint64(1), nil),
			// Tick 2: forward jump; the old tip is re-polled with delta hints.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r1", uint64(1)).
				Return(newTip, uint64(2), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r1", uint64(1)).
				Return(repoll, nil),
		)
		ds.EXPECT().Class(gomock.Any(), storedClass).Return(storedDef, nil)
		ds.EXPECT().Class(gomock.Any(), deltaClass).Return(deltaDef, nil)

		synctest.Test(t, func(t *testing.T) {
			h := wirePoller(t, fx.bc, fx.head, ds)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// Tick 1.
			time.Sleep(tickInterval)
			synctest.Wait()
			// Tick 2.
			time.Sleep(tickInterval)
			synctest.Wait()

			view := h.chain(t)
			classes := classesAt(&view, 1)
			require.Equal(t, storedDef, classes[*storedClass],
				"the stored tip's declared class must be recovered and fetched")
			require.Equal(t, deltaDef, classes[*deltaClass],
				"the delta's newly-declared class must be fetched")
		})
	})

	// A NoChange re-poll fetches the stored tip's declared classes, recovered from the
	// stored entry even though the poll carries no content.
	t.Run("no-change re-poll recovers stored tip classes", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		storedClass := new(felt.Felt).SetUint64(0xADD)
		storedDef := &core.SierraClass{}

		seed := makeTestPreConfirmedBlock("r1", 1) // stored old tip declares storedClass
		seed.TransactionStateDiffs[0].DeclaredClasses = []struct {
			ClassHash         *felt.Felt `json:"class_hash"`
			CompiledClassHash *felt.Felt `json:"compiled_class_hash"`
		}{{ClassHash: storedClass}}
		newTip := makeTestPreConfirmedBlock("r2", 0)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockStarknetData(ctrl)
		gomock.InOrder(
			// Tick 1: seed the old tip.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(seed, uint64(1), nil),
			// Tick 2: forward jump; the old tip re-poll reports no change.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r1", uint64(1)).
				Return(newTip, uint64(2), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r1", uint64(1)).
				Return(starknet.PreConfirmedNoChange{}, nil),
		)
		ds.EXPECT().Class(gomock.Any(), storedClass).Return(storedDef, nil)

		synctest.Test(t, func(t *testing.T) {
			h := wirePoller(t, fx.bc, fx.head, ds)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// Tick 1.
			time.Sleep(tickInterval)
			synctest.Wait()
			// Tick 2.
			time.Sleep(tickInterval)
			synctest.Wait()

			view := h.chain(t)
			require.Equal(t, storedDef, classesAt(&view, 1)[*storedClass],
				"the stored tip's declared class must be recovered despite a NoChange re-poll")
		})
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
		sub := h.poller.Subscribe()
		t.Cleanup(sub.Unsubscribe)

		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()

		select {
		case pc := <-sub.Recv():
			require.NotNil(t, pc)
			require.Equal(t, uint64(1), pc.Block.Number)
			require.Equal(t, "r0", pc.BlockIdentifier)
		default:
			t.Fatal("expected a pre_confirmed broadcast on successful apply")
		}
	})
}

// NoChange must not publish anything.
func TestPollerSilentOnNoChange(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	seed := makeTestPreConfirmedBlock("r0", 0)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockStarknetData(ctrl)
	gomock.InOrder(
		// Tick 1: seed the chain through the wire.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(seed, uint64(1), nil),
		// Tick 2: nothing changed.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
			Return(starknet.PreConfirmedNoChange{}, uint64(1), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		h := wirePoller(t, fx.bc, fx.head, ds)
		sub := h.poller.Subscribe()
		t.Cleanup(sub.Unsubscribe)

		go h.poller.Run(t.Context())
		synctest.Wait()

		// Tick 1 seeds through the wire, which publishes once; drain it so the
		// subscription starts tick 2 clean.
		time.Sleep(tickInterval)
		synctest.Wait()
		select {
		case pc := <-sub.Recv():
			require.Equal(t, uint64(1), pc.Block.Number)
			require.Equal(t, "r0", pc.BlockIdentifier)
		default:
			t.Fatal("expected the seeding tick to broadcast")
		}

		// Tick 2: NoChange.
		time.Sleep(tickInterval)
		synctest.Wait()
		select {
		case pc := <-sub.Recv():
			t.Fatalf("did not expect a broadcast on NoChange, got %+v", pc)
		default:
		}
	})
}
