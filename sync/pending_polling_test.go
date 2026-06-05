package sync

import (
	"context"
	"errors"
	stdsync "sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var feltOne = &felt.One

// MockDataSource provides controllable behaviour for polling functions.
type MockDataSource struct {
	DataSource
	pendingErrorThreshold      uint
	preConfirmedErrorThreshold uint
	numCallsPreConfirmed       uint
	numCallsPending            uint
	PendingFunc                func(ctx context.Context) (pending.PreLatest, error)
	PreConfirmedFunc           func(
		ctx context.Context,
		number uint64,
		blockIdentifier string,
		knownTransactionCount uint64,
		numCalls uint,
	) (starknet.PreConfirmedUpdate, error)
}

// Override BlockPreLatest to simulate errors and/or injected responses
func (m *MockDataSource) BlockPreLatest(ctx context.Context) (pending.PreLatest, error) {
	m.numCallsPending += 1
	if m.numCallsPending <= m.pendingErrorThreshold {
		return pending.PreLatest{}, errors.New("some error")
	}
	// fallback to embedded real method if no override set
	if m.PendingFunc != nil {
		return m.PendingFunc(ctx)
	}
	return m.DataSource.BlockPreLatest(ctx)
}

// Override PreConfirmedBlockByNumber to simulate errors and variable tx count,
// or injected responses.
func (m *MockDataSource) PreConfirmedBlockByNumber(
	ctx context.Context,
	number uint64,
	blockIdentifier string,
	knownTransactionCount uint64,
) (starknet.PreConfirmedUpdate, error) {
	m.numCallsPreConfirmed += 1
	if m.numCallsPreConfirmed <= m.preConfirmedErrorThreshold {
		return nil, errors.New("some error")
	}

	// fallback to embedded real method if no override set
	if m.PreConfirmedFunc != nil {
		return m.PreConfirmedFunc(
			ctx,
			number,
			blockIdentifier,
			knownTransactionCount,
			m.numCallsPreConfirmed,
		)
	}
	// Mirrors the old fallback's behaviour: each poll returns a richer block
	// than the last (count grows with numCalls) so the storage's
	// preserve-if-richer rotation actually rotates.
	txCount := int(number%10 + uint64(m.numCallsPreConfirmed)/2)
	return makeTestPreConfirmedBlock("mock", txCount), nil
}

// makeTestPreConfirmedBlock returns a starknet.PreConfirmedBlock carrying
// `txCount` synthesised invoke transactions with matching receipts and per-tx
// state diffs. Metadata (status, timestamp, sequencer, prices, DA mode) mirrors
// the realistic-looking fixture the old `makeTestPreConfirmed` helper produced.
//
// The block number is NOT carried by PreConfirmedBlock — it is provided
// separately to AdaptPreConfirmedBlock by the caller.
func makeTestPreConfirmedBlock(identifier string, txCount int) starknet.PreConfirmedBlock {
	txs := make([]starknet.Transaction, txCount)
	receipts := make([]*starknet.TransactionReceipt, txCount)
	stateDiffs := make([]*starknet.StateDiff, txCount)
	for i := range txCount {
		// Distinct, deterministic hash per tx so adaption is unambiguous.
		hash := new(felt.Felt).SetUint64(uint64(i + 1))
		emptySlice := []*felt.Felt{}
		txs[i] = starknet.Transaction{
			Hash:      hash,
			Type:      starknet.TxnInvoke,
			Version:   new(felt.Felt).SetUint64(1),
			CallData:  &emptySlice,
			Signature: &emptySlice,
		}
		receipts[i] = &starknet.TransactionReceipt{TransactionHash: hash}
		// One storage write per tx, keyed by the tx hash so state diffs aggregate
		// to N distinct entries when squashed — exercises the merge path
		// non-trivially in storage tests rather than coming out as a no-op.
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

func TestPollPreLatest(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	client := feeder.NewTestClient(t, &networks.Mainnet)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	logger := log.NewNopZapLogger()

	t.Run("Parent matches: immediate forward on tick", func(t *testing.T) {
		head, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)

		mockDS := &MockDataSource{
			DataSource: dataSource,
			PendingFunc: func(context.Context) (pending.PreLatest, error) {
				return makeEmptyPreLatestForParent(head.Header), nil
			},
		}

		s := New(bc, mockDS, logger, 50*time.Millisecond, 100*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(head.Header)

		preLatestCh := make(chan *pending.PreLatest, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg stdsync.WaitGroup

		wg.Go(func() {
			s.pollPreLatest(ctx, preLatestCh)
		})

		defer wg.Wait()
		time.Sleep(100 * time.Millisecond)
		// Send the head that matches the parent of pending
		s.newHeads.Send(head)

		select {
		case got := <-preLatestCh:
			require.NotNil(t, got)
			require.Equal(t, head.Number+1, got.Block.Number)
			require.NotNil(t, got.Block.ParentHash)
			require.Equal(t, *head.Hash, *got.Block.ParentHash)
		case <-ctx.Done():
			t.Fatal("expected pre-latest to be emitted")
		}
	})

	t.Run("Cache future pre_latest: emit when matching parent head arrives", func(t *testing.T) {
		latest, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)
		latestMinusOne, err := gw.BlockByNumber(t.Context(), latest.Number-1)
		require.NoError(t, err)

		mockDS := &MockDataSource{
			DataSource: dataSource,
			PendingFunc: func(context.Context) (pending.PreLatest, error) {
				// Always return pending for 'latest' (future relative to headMinusOne)
				return makeEmptyPreLatestForParent(latest.Header), nil
			},
		}
		s := New(bc, mockDS, logger, 50*time.Millisecond, 100*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(latestMinusOne.Header)

		preLatestCh := make(chan *pending.PreLatest, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPreLatest(ctx, preLatestCh)
		})

		defer wg.Wait()
		time.Sleep(100 * time.Millisecond)
		// First head: cache should be filled, but nothing emitted
		s.newHeads.Send(latestMinusOne)

		select {
		case <-preLatestCh:
			t.Fatal("did not expect a pre-latest for future-parent case")
		case <-time.After(200 * time.Millisecond):
		}

		// Now send the matching head; cached pre-latest should be emitted immediately
		s.newHeads.Send(latest)

		select {
		case got := <-preLatestCh:
			require.NotNil(t, got)
			require.Equal(t, latest.Number+1, got.Block.Number, "number should be adjusted to parent head+1")
			require.Equal(t, *latest.Hash, *got.Block.ParentHash)
		case <-ctx.Done():
			t.Fatal("expected cached pre-latest to be forwarded when its parent head arrives")
		}
	})

	t.Run("Retry on error then success", func(t *testing.T) {
		head, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)

		mockDS := &MockDataSource{
			DataSource:            dataSource,
			pendingErrorThreshold: 2,
			PendingFunc: func(context.Context) (pending.PreLatest, error) {
				return makeEmptyPreLatestForParent(head.Header), nil
			},
			preConfirmedErrorThreshold: 0,
		}

		s := New(bc, mockDS, logger, 50*time.Millisecond, 100*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(head.Header)

		preLatestCh := make(chan *pending.PreLatest, 2)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var wg stdsync.WaitGroup

		wg.Go(func() {
			s.pollPreLatest(ctx, preLatestCh)
		})
		defer wg.Wait()
		time.Sleep(100 * time.Millisecond)
		s.newHeads.Send(head)

		select {
		case got := <-preLatestCh:
			require.NotNil(t, got)
			require.GreaterOrEqual(t, mockDS.numCallsPending, uint(3), "expected at least 3 attempts (2 errors + 1 success)")
			require.Equal(t, head.Number+1, got.Block.Number)
		case <-ctx.Done():
			t.Fatal("expected pre-latest after retries")
		}
	})

	t.Run("Always error then respect cancel", func(t *testing.T) {
		head, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)

		mockDS := &MockDataSource{
			DataSource:            dataSource,
			pendingErrorThreshold: ^uint(0), // always error
			PendingFunc: func(context.Context) (pending.PreLatest, error) {
				return makeEmptyPreLatestForParent(head.Header), nil
			},
		}

		s := New(bc, mockDS, logger, 50*time.Millisecond, 100*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(head.Header)

		preLatestCh := make(chan *pending.PreLatest, 2)
		ctx, cancel := context.WithCancel(context.Background())

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPreLatest(ctx, preLatestCh)
		})

		s.newHeads.Send(head)

		// Ensure nothing arrives for some time
		select {
		case <-preLatestCh:
			t.Fatal("unexpected pre-latest")
		case <-time.After(300 * time.Millisecond):
		}

		// Cancel and ensure routine exits cleanly
		cancel()
		wg.Wait()
	})
}

func TestPollPreConfirmedLoop(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Sepolia,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	client := feeder.NewTestClient(t, &networks.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	logger := log.NewNopZapLogger()

	// Store block 0 as head
	head0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)
	require.NoError(t, bc.Store(
		head0,
		&core.BlockCommitments{},
		stateUpdate0,
		map[felt.Felt]core.ClassDefinition{},
	))

	t.Run("Skips when no target, polls when target set and at tip; retries on error then success", func(t *testing.T) {
		mockDS := &MockDataSource{
			DataSource:                 dataSource,
			preConfirmedErrorThreshold: 2,
		}
		s := New(bc, mockDS, logger, 0, 30*time.Millisecond, false, testDB)

		var preConfirmedBlockNumberToPoll atomic.Uint64
		out := make(chan preConfirmedPoll, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, out)
		})
		defer wg.Wait()

		// Initially, no target -> should not emit
		select {
		case <-out:
			t.Fatal("unexpected pre_confirmed with no target set")
		case <-time.After(100 * time.Millisecond):
		}

		// Set target but not at tip (highest nil) -> still skip
		preConfirmedBlockNumberToPoll.Store(uint64(1))
		select {
		case <-out:
			t.Fatal("unexpected pre_confirmed when not at tip")
		case <-time.After(100 * time.Millisecond):
		}

		// Now set highest header to 0, which makes isAtTip(1) true
		s.highestBlockHeader.Store(head0.Header)

		select {
		case poll := <-out:
			require.Equal(t, uint64(1), poll.blockNumber)
			_, ok := poll.update.(starknet.PreConfirmedBlock)
			require.True(t, ok, "expected PreConfirmedBlock, got %T", poll.update)
			require.GreaterOrEqual(
				t,
				mockDS.numCallsPreConfirmed,
				uint(3),
				"expected at least 3 attempts (2 errors + 1 success)",
			)
		case <-ctx.Done():
			t.Fatal("did not receive pre_confirmed after setting target and being at tip")
		}
	})

	t.Run("Respects context cancel on continuous errors", func(t *testing.T) {
		mockDS := &MockDataSource{
			DataSource:                 dataSource,
			preConfirmedErrorThreshold: ^uint(0), // never stop erroring
		}
		s := New(bc, mockDS, logger, 0, 30*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(head0.Header)
		var preConfirmedBlockNumberToPoll atomic.Uint64
		preConfirmedBlockNumberToPoll.Store(1)

		out := make(chan preConfirmedPoll, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, out)
		})
		wg.Wait()

		require.GreaterOrEqual(t, mockDS.numCallsPreConfirmed, uint(1), "Should have retried at least once before context cancelled")
	})
}

func TestPollPendingData(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Sepolia)
	gw := adaptfeeder.New(client)
	logger := log.NewNopZapLogger()

	fetchBlock := func(number uint64) (*core.Block, *core.StateUpdate) {
		block, err := gw.BlockByNumber(t.Context(), number)
		require.NoError(t, err)
		stateUpdate, err := gw.StateUpdate(t.Context(), number)
		require.NoError(t, err)
		return block, stateUpdate
	}

	newBC := func() *blockchain.Blockchain {
		return blockchain.New(
			memory.New(),
			&networks.Sepolia,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)
	}

	t.Run("preConfirmed target advances when preLatest arrives", func(t *testing.T) {
		bc := newBC()
		testDB := memory.New()
		dataSource := NewFeederGatewayDataSource(bc, gw)
		block0, stateUpdate0 := fetchBlock(0)
		storeBlock(t, bc, block0, stateUpdate0)

		mockDataSource := &MockDataSource{
			DataSource:            dataSource,
			pendingErrorThreshold: 2, // delay before pre_latest succeeds
			PendingFunc: func(context.Context) (pending.PreLatest, error) {
				head, err := bc.HeadsHeader()
				require.NoError(t, err)
				return makeEmptyPreLatestForParent(head), nil
			},
		}
		s := New(bc, mockDataSource, logger, 50*time.Millisecond, 50*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(block0.Header)

		sub := s.preConfirmedDataFeed.SubscribeKeepLast()
		defer sub.Unsubscribe()

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		var wg stdsync.WaitGroup
		wg.Go(func() { s.pollPendingData(ctx) })
		defer wg.Wait()

		time.Sleep(100 * time.Millisecond)
		s.newHeads.Send(block0)

		// First broadcast must be the baseline pre_confirmed for head+1 = 1.
		first := waitForBroadcast(t, sub, ctx)
		require.Equal(t, uint64(1), first.GetBlock().Number)

		// Once pre_latest stops erroring, target should bump to 2.
		waitForBlockNumber(t, sub, ctx, 2)
	})

	t.Run("scripted poll sequence: Full / NoChange / Delta / new-identifier", func(t *testing.T) {
		// Blocks must be fetched outside the synctest goroutine.
		block0, stateUpdate0 := fetchBlock(0)
		block1, stateUpdate1 := fetchBlock(1)
		bc := newBC()
		testDB := memory.New()
		dataSource := NewFeederGatewayDataSource(bc, gw)

		const preCPoll = 500 * time.Millisecond
		const preLPoll = time.Second

		// Each entry is the response for poll N (1-indexed). The scenario is:
		// height 1 — Full → NoChange.
		// (head advances to 1)
		// height 2 — Full → Delta(+1 tx) → Full(new identifier) → NoChange.
		emptyFelts := []*felt.Felt{}
		script := []starknet.PreConfirmedUpdate{
			makeTestPreConfirmedBlock("block1-id", 1),
			starknet.PreConfirmedNoChange{},
			makeTestPreConfirmedBlock("block2-id", 0),
			starknet.PreConfirmedDeltaUpdate{
				BlockIdentifier: "block2-id",
				Transactions: []starknet.Transaction{
					{Type: starknet.TxnInvoke, CallData: &emptyFelts, Signature: &emptyFelts},
				},
				Receipts:              []*starknet.TransactionReceipt{{}},
				TransactionStateDiffs: []*starknet.StateDiff{{}},
			},
			makeTestPreConfirmedBlock("0xdeadbeef", 0),
			starknet.PreConfirmedNoChange{},
		}

		synctest.Test(t, func(t *testing.T) {
			mockDS := &MockDataSource{
				DataSource: dataSource,
				PendingFunc: func(context.Context) (pending.PreLatest, error) {
					return pending.PreLatest{}, errors.New("no PreLatest available")
				},
				PreConfirmedFunc: func(
					_ context.Context, _ uint64, _ string, _ uint64, numCalls uint,
				) (starknet.PreConfirmedUpdate, error) {
					if int(numCalls) > len(script) {
						return starknet.PreConfirmedNoChange{}, nil
					}
					return script[numCalls-1], nil
				},
			}

			s := New(bc, mockDS, logger, preLPoll, preCPoll, false, testDB)
			sub := s.preConfirmedDataFeed.SubscribeKeepLast()
			defer sub.Unsubscribe()

			go func() { s.pollPendingData(t.Context()) }()
			synctest.Wait()

			advance := func(b *core.Block, su *core.StateUpdate) {
				storeBlock(t, bc, b, su)
				s.highestBlockHeader.Store(b.Header)
				s.newHeads.Send(b)
				synctest.Wait()
			}
			tick := func() {
				time.Sleep(preCPoll)
				synctest.Wait()
			}

			advance(block0, stateUpdate0) // target := 1

			// poll 1: Full → broadcast at height 1.
			tick()
			pc1 := expectBroadcast(t, sub)
			assert.Equal(t, uint64(1), pc1.Block.Number)

			// poll 2: NoChange → no broadcast.
			tick()
			expectNoBroadcast(t, sub)

			advance(block1, stateUpdate1) // target := 2

			// poll 3: Full → broadcast at height 2.
			tick()
			pc2 := expectBroadcast(t, sub)
			assert.Equal(t, uint64(2), pc2.Block.Number)

			// poll 4: Delta enriches existing block → broadcast.
			tick()
			deltaPc := expectBroadcast(t, sub)
			assert.Equal(t, pc2.Block.Number, deltaPc.Block.Number)
			assert.NotEqual(t, pc2, deltaPc)

			// poll 5: Full with new identifier replaces existing → broadcast.
			tick()
			replaced := expectBroadcast(t, sub)
			assert.Equal(t, deltaPc.Block.Number, replaced.Block.Number)
			assert.NotEqual(t, deltaPc.BlockIdentifier, replaced.BlockIdentifier)

			// poll 6: NoChange → no broadcast.
			tick()
			expectNoBroadcast(t, sub)
		})
	})
}

// storeBlock writes a block + state update with empty commitments and class map.
func storeBlock(t *testing.T, bc *blockchain.Blockchain, b *core.Block, su *core.StateUpdate) {
	t.Helper()
	require.NoError(t, bc.Store(b, &core.BlockCommitments{}, su, map[felt.Felt]core.ClassDefinition{}))
}

// waitForBroadcast reads the next pre_confirmed broadcast or fails on ctx done.
func waitForBroadcast(
	t *testing.T,
	sub *feed.Subscription[*pending.PreConfirmed],
	ctx context.Context,
) *pending.PreConfirmed {
	t.Helper()
	select {
	case pc := <-sub.Recv():
		require.NotNil(t, pc)
		return pc
	case <-ctx.Done():
		t.Fatal("expected a pre_confirmed broadcast but context expired")
		return nil
	}
}

// waitForBlockNumber drains broadcasts until one with the target block number
// arrives. Fails if a later block number is observed first (skip) or ctx ends.
func waitForBlockNumber(
	t *testing.T,
	sub *feed.Subscription[*pending.PreConfirmed],
	ctx context.Context,
	target uint64,
) {
	t.Helper()
	for {
		pc := waitForBroadcast(t, sub, ctx)
		switch {
		case pc.GetBlock().Number == target:
			return
		case pc.GetBlock().Number > target:
			t.Fatalf("skipped pre_confirmed for number %d (saw %d)", target, pc.GetBlock().Number)
		}
	}
}

// expectBroadcast non-blockingly reads one broadcast under synctest virtual
// time; the caller must have advanced time and called synctest.Wait first.
func expectBroadcast(
	t *testing.T, sub *feed.Subscription[*pending.PreConfirmed],
) *pending.PreConfirmed {
	t.Helper()
	select {
	case pc := <-sub.Recv():
		require.NotNil(t, pc)
		return pc
	default:
		t.Fatal("expected a pre_confirmed broadcast")
		return nil
	}
}

// expectNoBroadcast asserts that no broadcast is pending; the caller must have
// advanced time and called synctest.Wait first.
func expectNoBroadcast(t *testing.T, sub *feed.Subscription[*pending.PreConfirmed]) {
	t.Helper()
	select {
	case <-sub.Recv():
		t.Fatal("did not expect a pre_confirmed broadcast")
	default:
	}
}

func makeEmptyPreLatestForParent(parent *core.Header) pending.PreLatest {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       parent.Hash,
			SequencerAddress: parent.SequencerAddress,
			Number:           parent.Number + 1,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  core.Ver0_14_0.String(),
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    parent.L1GasPriceETH,
			L1GasPriceSTRK:   parent.L1GasPriceSTRK,
			L2GasPrice:       parent.L2GasPrice,
			L1DataGasPrice:   parent.L1DataGasPrice,
			L1DAMode:         parent.L1DAMode,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}
	stateDiff := core.EmptyStateDiff()
	pending := pending.PreLatest{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   parent.GlobalStateRoot,
			StateDiff: &stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.ClassDefinition, 0),
	}
	return pending
}
