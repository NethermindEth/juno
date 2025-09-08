package sync

import (
	"context"
	"errors"
	stdsync "sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

type MockDataSource struct {
	DataSource

	pendingErrorThreshold      uint
	preConfirmedErrorThreshold uint
	numCallsPreConfirmed       uint
	numCallsPending            uint
	PendingFunc                func(ctx context.Context) (core.Pending, error)
}

// Override BlockPending
func (m *MockDataSource) BlockPending(ctx context.Context) (core.Pending, error) {
	m.numCallsPending += 1
	if m.numCallsPending <= m.pendingErrorThreshold {
		return core.Pending{}, errors.New("some error")
	}
	// fallback to embedded real method if no override set
	if m.PendingFunc != nil {
		return m.PendingFunc(ctx)
	}
	return m.DataSource.BlockPending(ctx)
}

// Override PreConfirmedBlockByNumber
func (m *MockDataSource) PreConfirmedBlockByNumber(ctx context.Context, number uint64) (core.PreConfirmed, error) {
	m.numCallsPreConfirmed += 1
	if m.numCallsPreConfirmed <= m.preConfirmedErrorThreshold {
		return core.PreConfirmed{}, errors.New("some error")
	}

	preConfirmed := makeTestPreConfirmed(number)
	preConfirmed.Block.TransactionCount = number%10 + uint64(m.numCallsPreConfirmed)/2

	return preConfirmed, nil
}

func TestFetchPreLatest(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	log := utils.NewNopZapLogger()

	s := New(bc, dataSource, log, 100*time.Millisecond, 100*time.Millisecond, false, testDB)

	t.Run("Parent matches immediate forward", func(t *testing.T) {
		head, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)
		s.highestBlockHeader.Store(head.Header)

		seen := make(map[felt.Felt]*core.PreLatest)
		ch := make(chan *core.PreLatest, 1)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		s.fetchPreLatest(ctx, head, seen, ch)

		select {
		case got := <-ch:
			require.NotNil(t, got)
			require.Equal(t, head.Number+1, got.Block.Number, "pre-latest number should be head.Number+1")
			require.NotNil(t, got.Block.ParentHash)
			require.Equal(t, *head.Hash, *got.Block.ParentHash, "parent hash should match head hash")
		default:
			t.Fatal("expected pre-latest")
		}
		require.Empty(t, seen, "cache should remove entry when parent matches")
	})

	t.Run("Cache future PreLatest emit on parent head", func(t *testing.T) {
		latest, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)

		latestMinusOne, err := gw.BlockByNumber(t.Context(), latest.Number-1)
		require.NoError(t, err)

		s.highestBlockHeader.Store(latestMinusOne.Header)

		seen := make(map[felt.Felt]*core.PreLatest)
		ch := make(chan *core.PreLatest, 1)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// First call: should cache and not forward
		s.fetchPreLatest(ctx, latestMinusOne, seen, ch)
		select {
		case <-ch:
			t.Fatal("did not expect a pre-latest to be forwarded for future-parent case")
		default:
			// ok
		}
		// Cache should have exactly one entry
		if _, ok := seen[*latest.Hash]; !ok || len(seen) != 1 {
			t.Fatalf("expected cache to contain exactly one entry for latest; got %d", len(seen))
		}

		// Second call: should emit from cache without calling BlockPending again
		s.fetchPreLatest(ctx, latest, seen, ch)

		select {
		case got := <-ch:
			require.NotNil(t, got)
			require.Equal(t, latest.Number+1, got.Block.Number, "cached pre-latest number should be adjusted to parent head+1")
		default:
			t.Fatal("expected cached pre-latest to be forwarded when its parent head arrives")
		}
		require.Empty(t, seen, "cache should be empty after emitting cached pre-latest")
	})

	t.Run("Retry on error then success", func(t *testing.T) {
		head, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)

		mockDataSource := MockDataSource{DataSource: dataSource, pendingErrorThreshold: 2}
		s := New(bc, &mockDataSource, log, 50*time.Millisecond, 0, false, testDB)
		s.highestBlockHeader.Store(head.Header)

		seen := make(map[felt.Felt]*core.PreLatest)
		ch := make(chan *core.PreLatest, 1)

		ctxA, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		s.fetchPreLatest(ctxA, head, seen, ch)
		select {
		case got := <-ch:
			require.NotNil(t, got)
			require.GreaterOrEqual(t, mockDataSource.numCallsPending, uint(3), "expected at least 3 attempts (2 errors + 1 success)")
			require.Equal(t, head.Number+1, got.Block.Number)
		default:
			t.Fatal("expected pre-latest")
		}
	})

	t.Run("Always error then respect cancel", func(t *testing.T) {
		head, err := gw.BlockLatest(t.Context())
		require.NoError(t, err)

		mockDataSource := MockDataSource{DataSource: dataSource, pendingErrorThreshold: ^uint(0)}
		s := New(bc, &mockDataSource, log, 50*time.Millisecond, 0, false, testDB)
		s.highestBlockHeader.Store(head.Header)

		seen := make(map[felt.Felt]*core.PreLatest)
		ch := make(chan *core.PreLatest, 1)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		timeout := time.After(200 * time.Millisecond)
		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.fetchPreLatest(ctx, head, seen, ch)
		})
		select {
		case <-ch:
			t.Fatal("unexpected pre-latest")
		case <-timeout:
			cancel()
			wg.Wait()
		}
	})
}

func TestFetchAndStorePreConfirmed(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Sepolia)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	log := utils.NewNopZapLogger()

	head, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	require.NoError(t, bc.Store(
		head,
		&core.BlockCommitments{},
		stateUpdate0,
		map[felt.Felt]core.Class{},
	))

	s := New(bc, dataSource, log, 0, 50*time.Millisecond, false, testDB)

	preConfirmedBlockNumber := uint64(1)
	t.Run("Highest head skip case: returns immediately if highestBlockHeader is nil", func(t *testing.T) {
		s.highestBlockHeader.Store(nil)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		done := make(chan struct{})
		go func() {
			s.fetchAndStorePreConfirmed(ctx, preConfirmedBlockNumber, nil)
			close(done)
		}()
		select {
		case <-done:
			// OK
		case <-ctx.Done():
			t.Fatal("did not return immediately when highestBlockHeader is nil")
		}
	})

	t.Run("Highest head skip case: returns immediately if already past the tip", func(t *testing.T) {
		s.highestBlockHeader.Store(&core.Header{Number: preConfirmedBlockNumber + 1})
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		done := make(chan struct{})
		go func() {
			s.fetchAndStorePreConfirmed(ctx, preConfirmedBlockNumber, nil)
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("did not return immediately when block height skipped")
		}
	})

	t.Run("Error waiting case: retry until success", func(t *testing.T) {
		mockDataSource := &MockDataSource{
			DataSource:                 dataSource,
			preConfirmedErrorThreshold: 2,
		}
		s := New(bc, mockDataSource, log, 0, 50*time.Millisecond, false, testDB)

		s.highestBlockHeader.Store(&core.Header{Number: preConfirmedBlockNumber - 1})

		// Subscribe to pending data for broadcast
		sub := s.SubscribePendingData()
		defer sub.Unsubscribe()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.fetchAndStorePreConfirmed(ctx, preConfirmedBlockNumber, nil)
		})

		select {
		case pending := <-sub.Recv():
			require.NotNil(t, pending, "Should receive a pendingData broadcast")
			require.Equal(t, preConfirmedBlockNumber, pending.GetBlock().Number)
			// Should call PreConfirmedBlockByNumber at least errorThreshold+1 times (retries then success)
			require.GreaterOrEqual(t, mockDataSource.numCallsPreConfirmed, uint(3))
			require.Equal(t, pending, *s.pendingData.Load())
		case <-ctx.Done():
			t.Fatal("Did not receive pendingData in time (error case with retries)")
		}
		cancel()
		wg.Wait()
	})

	t.Run("respect context cancel", func(t *testing.T) {
		mockDataSource := &MockDataSource{
			DataSource:                 dataSource,
			preConfirmedErrorThreshold: ^uint(0), // never stop erroring
		}
		s := New(bc, mockDataSource, log, 0, 50*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(&core.Header{Number: preConfirmedBlockNumber - 1})

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.fetchAndStorePreConfirmed(ctx, preConfirmedBlockNumber, nil)
		})

		wg.Wait()
		// Expect lots of calls, but no panics and returns after cancel
		require.GreaterOrEqual(t, mockDataSource.numCallsPreConfirmed, uint(1), "Should have retried at least once")
	})
}

func TestStorePreConfirmed(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Mainnet)
	log := utils.NewNopZapLogger()

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	s := New(bc, NewFeederGatewayDataSource(bc, nil), log, 0, 0, false, testDB)
	t.Run("stores pre_confirmed when there is none (first entry)", func(t *testing.T) {
		preConfirmed := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{Number: 0},
			},
			StateUpdate: &core.StateUpdate{},
		}
		t.Run("head is nil", func(t *testing.T) {
			written, err := s.StorePreConfirmed(preConfirmed)
			require.NoError(t, err)
			require.True(t, written)
			ptr := s.pendingData.Load()
			require.NotNil(t, ptr)
			require.Equal(t, preConfirmed, *ptr)
		})

		head, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		require.NoError(t, bc.Store(
			head,
			&core.BlockCommitments{},
			stateUpdate0,
			map[felt.Felt]core.Class{},
		))

		t.Run("not valid for head", func(t *testing.T) {
			s.pendingData.Store(nil)

			written, err := s.StorePreConfirmed(preConfirmed)
			require.Error(t, err)
			require.False(t, written)
		})
	})

	t.Run("returns error if ProtocolVersion unsupported", func(t *testing.T) {
		pc := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{Number: 1, ProtocolVersion: "1.9.0"},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err := s.StorePreConfirmed(pc)
		require.Error(t, err)
		require.False(t, written)
	})

	t.Run("overwrites if existing pending is invalid", func(t *testing.T) {
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		invalidPreConfirmed := &core.PreConfirmed{
			Block:       &core.Block{Header: &core.Header{Number: 0}},
			StateUpdate: &core.StateUpdate{},
		}
		// Insert invalid pending (simulate old data)
		s.pendingData.Store(utils.HeapPtr[core.PendingData](invalidPreConfirmed))
		pc := &core.PreConfirmed{
			Block:       &core.Block{Header: &core.Header{Number: head.Number + 1}},
			StateUpdate: &core.StateUpdate{},
		}

		written, err := s.StorePreConfirmed(pc)
		require.NoError(t, err)
		require.True(t, written)
	})

	t.Run("ignores pre_confirmed with fewer or equal txs for the same block number", func(t *testing.T) {
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		better := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 2,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}

		written, err := s.StorePreConfirmed(better)
		require.NoError(t, err)
		require.True(t, written)
		// Attempt to store worse
		worse := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 1,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err = s.StorePreConfirmed(worse)
		require.NoError(t, err)
		require.False(t, written)
		ptr := s.pendingData.Load()
		require.Equal(t, better, *ptr)
	})

	t.Run("accepts pre_confirmed with more txs for same block number", func(t *testing.T) {
		s.pendingData.Store(nil)
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		worse := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 1,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err := s.StorePreConfirmed(worse)
		require.NoError(t, err)
		require.True(t, written)
		ptr := s.pendingData.Load()
		require.Equal(t, worse, *ptr)

		better := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 2,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err = s.StorePreConfirmed(better)
		require.NoError(t, err)
		require.True(t, written)
		ptr = s.pendingData.Load()
		require.Equal(t, better, *ptr)
	})
}

func TestPollPreLatest(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	log := utils.NewNopZapLogger()

	s := New(bc, dataSource, log, 50*time.Millisecond, time.Second, false, testDB)
	s.highestBlockHeader.Store(&core.Header{Number: 0})
	preLatestCh := make(chan *core.PreLatest, 2)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	var wg stdsync.WaitGroup
	wg.Go(func() {
		s.pollPreLatest(ctx, preLatestCh)
	})
	defer wg.Wait()

	head, err := gw.BlockLatest(t.Context())
	require.NoError(t, err)
	s.newHeads.Send(head)

	select {
	case pre := <-preLatestCh:
		require.NotNil(t, pre)
		require.Equal(t, head.Number+1, pre.Block.Number)
	case <-ctx.Done():
		t.Fatal("did not emit preLatest on head")
	}
}

func TestPollPreConfirmed(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Sepolia)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)

	pendingFunc := func(context.Context) (core.Pending, error) {
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		return makeTestPendingForParent(head), nil
	}

	log := utils.NewNopZapLogger()
	mockDataSource := &MockDataSource{
		DataSource:            dataSource,
		PendingFunc:           pendingFunc,
		pendingErrorThreshold: 5, // introduce delay to receive pre_confirmed for head
	}
	s := New(bc, mockDataSource, log, 50*time.Millisecond, 50*time.Millisecond, false, testDB)

	fetchStoreBlock := func(t *testing.T, blockNumber uint64) *core.Block {
		t.Helper()
		block, err := gw.BlockByNumber(t.Context(), blockNumber)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NoError(t, bc.Store(
			block,
			&core.BlockCommitments{},
			stateUpdate0,
			map[felt.Felt]core.Class{},
		))
		s.highestBlockHeader.Store(block.Header)
		return block
	}

	sub := s.pendingDataFeed.SubscribeKeepLast()
	defer sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var wg stdsync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		s.pollPreConfirmed(ctx)
	})
	time.Sleep(50 * time.Millisecond)
	head := fetchStoreBlock(t, 0)
	s.newHeads.Send(head)

	// receive pre_confirmed for head
	select {
	case preConfirmed := <-sub.Recv():
		require.NotNil(t, preConfirmed)
		require.Equal(t, uint64(1), preConfirmed.GetBlock().Number)
	case <-ctx.Done():
		t.Fatal("did not broadcast preConfirmed")
	}

	time.Sleep(300 * time.Millisecond)
	// receive pre_confirmed for pre_latest
	select {
	case preConfirmed := <-sub.Recv():
		require.NotNil(t, preConfirmed)
		require.Equal(t, uint64(2), preConfirmed.GetBlock().Number)
	case <-ctx.Done():
		t.Fatal("did not broadcast preConfirmed")
	}
}

func TestPollPendingDataSwitchToPreConfirmedPolling(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Sepolia)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)

	// Returns pending block with protocol version 0.14.0
	pendingFunc := func(context.Context) (core.Pending, error) {
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		return makeTestPendingForParent(head), nil
	}

	log := utils.NewNopZapLogger()
	mockDataSource := &MockDataSource{
		DataSource:  dataSource,
		PendingFunc: pendingFunc,
	}
	s := New(bc, mockDataSource, log, 50*time.Millisecond, 50*time.Millisecond, false, testDB)

	fetchStoreBlock := func(t *testing.T, blockNumber uint64) *core.Block {
		t.Helper()
		block, err := gw.BlockByNumber(t.Context(), blockNumber)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NoError(t, bc.Store(
			block,
			&core.BlockCommitments{},
			stateUpdate0,
			map[felt.Felt]core.Class{},
		))
		s.highestBlockHeader.Store(block.Header)
		return block
	}

	head := fetchStoreBlock(t, 0)
	sub := s.pendingDataFeed.SubscribeKeepLast()
	defer sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var wg stdsync.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		s.pollPendingData(ctx)
	})
	time.Sleep(100 * time.Millisecond)
	s.newHeads.Send(head)
	// wait for pre_latest
	time.Sleep(100 * time.Millisecond)
	// receive pre_confirmed for head
	select {
	case preConfirmed := <-sub.Recv():
		require.NotNil(t, preConfirmed)
		require.Equal(t, core.PreConfirmedBlockVariant, preConfirmed.Variant())
		require.Equal(t, uint64(2), preConfirmed.GetBlock().Number)
	case <-ctx.Done():
		t.Fatal("did not broadcast preConfirmed")
	}
}

func makeTestPreConfirmed(num uint64) core.PreConfirmed {
	receipts := make([]*core.TransactionReceipt, 0)
	preConfirmedBlock := &core.Block{
		// pre_confirmed block does not have parent hash
		Header: &core.Header{
			SequencerAddress: &felt.One,
			Number:           num,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  core.Ver0_14_0.String(),
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    feltOne,
			L1GasPriceSTRK:   feltOne,
			L2GasPrice: &core.GasPrice{
				PriceInWei: feltOne,
				PriceInFri: feltOne,
			},
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: feltOne,
				PriceInFri: feltOne,
			},
			L1DAMode: core.Blob,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff := core.EmptyStateDiff()
	preConfirmed := core.PreConfirmed{
		Block: preConfirmedBlock,
		StateUpdate: &core.StateUpdate{
			StateDiff: &stateDiff,
		},
		NewClasses:            make(map[felt.Felt]core.Class, 0),
		TransactionStateDiffs: make([]*core.StateDiff, 0),
		CandidateTxs:          make([]core.Transaction, 0),
	}

	return preConfirmed
}

func makeTestPendingForParent(parent *core.Header) core.Pending {
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

	pending := core.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   parent.GlobalStateRoot,
			StateDiff: &stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}

	return pending
}
