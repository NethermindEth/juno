package sync

import (
	"context"
	"errors"
	stdsync "sync"
	"sync/atomic"
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

// MockDataSource provides controllable behaviour for polling functions.
type MockDataSource struct {
	DataSource
	pendingErrorThreshold      uint
	preConfirmedErrorThreshold uint
	numCallsPreConfirmed       uint
	numCallsPending            uint
	PendingFunc                func(ctx context.Context) (core.Pending, error)
}

// Override BlockPending to simulate errors and/or injected responses
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

// Override PreConfirmedBlockByNumber to simulate errors and variable tx count
func (m *MockDataSource) PreConfirmedBlockByNumber(ctx context.Context, number uint64) (core.PreConfirmed, error) {
	m.numCallsPreConfirmed += 1
	if m.numCallsPreConfirmed <= m.preConfirmedErrorThreshold {
		return core.PreConfirmed{}, errors.New("some error")
	}
	preConfirmed := makeTestPreConfirmed(number)
	preConfirmed.Block.TransactionCount = number%10 + uint64(m.numCallsPreConfirmed)/2
	return preConfirmed, nil
}

func TestPollPreConfirmedLoop(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Sepolia)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	log := utils.NewNopZapLogger()

	// Store block 0 as head
	head0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)
	require.NoError(t, bc.Store(
		head0,
		&core.BlockCommitments{},
		stateUpdate0,
		map[felt.Felt]core.Class{},
	))

	t.Run("Skips when no target, polls when target set and at tip; retries on error then success", func(t *testing.T) {
		mockDS := &MockDataSource{
			DataSource:                 dataSource,
			preConfirmedErrorThreshold: 2,
		}
		s := New(bc, mockDS, log, 0, 30*time.Millisecond, false, testDB)

		var preConfirmedBlockNumberToPoll atomic.Uint64
		out := make(chan *core.PreConfirmed, 1)
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
		case pc := <-out:
			require.NotNil(t, pc)
			require.Equal(t, uint64(1), pc.Block.Number)
			require.GreaterOrEqual(t, mockDS.numCallsPreConfirmed, uint(3), "expected at least 3 attempts (2 errors + 1 success)")
		case <-ctx.Done():
			t.Fatal("did not receive pre_confirmed after setting target and being at tip")
		}
	})

	t.Run("Respects context cancel on continuous errors", func(t *testing.T) {
		mockDS := &MockDataSource{
			DataSource:                 dataSource,
			preConfirmedErrorThreshold: ^uint(0), // never stop erroring
		}
		s := New(bc, mockDS, log, 0, 30*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(head0.Header)
		var preConfirmedBlockNumberToPoll atomic.Uint64
		preConfirmedBlockNumberToPoll.Store(1)

		out := make(chan *core.PreConfirmed, 1)
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

func TestRunPreConfirmedPhase(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &utils.Sepolia)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)
	log := utils.NewNopZapLogger()

	// Set up head 0
	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)
	require.NoError(t, bc.Store(
		block0,
		&core.BlockCommitments{},
		stateUpdate0,
		map[felt.Felt]core.Class{},
	))

	// Mock data source to delay pre_latest (pending) while allowing pre_confirmed to arrive
	pendingFunc := func(context.Context) (core.Pending, error) {
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		return makeTestPendingForParent(head), nil
	}

	mockDataSource := &MockDataSource{
		DataSource:            dataSource,
		PendingFunc:           pendingFunc,
		pendingErrorThreshold: 2, // introduce delay to receive pre_latest
	}
	s := New(bc, mockDataSource, log, 50*time.Millisecond, 50*time.Millisecond, false, testDB)
	s.highestBlockHeader.Store(block0.Header)

	// Subscribe to pending data feed to observe stored pre_confirmed
	sub := s.pendingDataFeed.SubscribeKeepLast()
	defer sub.Unsubscribe()

	// Create a heads subscription used by runPreConfirmedPhase
	headsSub := s.newHeads.SubscribeKeepLast()
	defer headsSub.Unsubscribe()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	var wg stdsync.WaitGroup
	wg.Go(func() {
		s.runPreConfirmedPhase(ctx, headsSub)
	})
	defer wg.Wait()

	time.Sleep(100 * time.Millisecond)
	// Send initial head 0 to kick off target=1
	s.newHeads.Send(block0)

	// Expect pre_confirmed for 1 first (baseline from head)
	select {
	case pd := <-sub.Recv():
		require.NotNil(t, pd)
		require.Equal(t, uint64(1), pd.GetBlock().Number)
	case <-ctx.Done():
		t.Fatal("did not broadcast pre_confirmed for number 1")
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

	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)
	require.NoError(t, bc.Store(
		block0,
		&core.BlockCommitments{},
		stateUpdate0,
		map[felt.Felt]core.Class{},
	))

	s.highestBlockHeader.Store(block0.Header)

	sub := s.pendingDataFeed.SubscribeKeepLast()
	defer sub.Unsubscribe()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	var wg stdsync.WaitGroup
	wg.Go(func() {
		s.pollPendingData(ctx)
	})

	time.Sleep(100 * time.Millisecond)
	s.newHeads.Send(block0)

	// receive pre_confirmed after switching to pre_confirmed polling
	select {
	case preConfirmed := <-sub.Recv():
		require.NotNil(t, preConfirmed)
		require.Equal(t, core.PreConfirmedBlockVariant, preConfirmed.Variant())
		// After switch and head=0, the first pre_confirmed target is either 1 or 2
		require.GreaterOrEqual(t, preConfirmed.GetBlock().Number, uint64(1))
	case <-ctx.Done():
		t.Fatal("did not broadcast pre_confirmed")
	}

	cancel()
	wg.Wait()
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
				Header: &core.Header{
					Number:          1,
					ProtocolVersion: blockchain.SupportedStarknetVersion.IncMajor().String(),
				},
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

		// Store "better" with higher tx count
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
		require.NotNil(t, ptr)
		stored, ok := (*ptr).(*core.PreConfirmed)
		require.True(t, ok)
		require.Equal(t, better, stored)
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
