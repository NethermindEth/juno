package sync

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	numCallsPreConfirmed       atomic.Uint32
	numCallsPending            atomic.Uint32
	PendingFunc                func(ctx context.Context) (pending.PreLatest, error)
	PreConfirmedFunc           func(
		ctx context.Context,
		number uint64,
		blockIdentifier string,
		knownTransactionCount uint64,
		numCalls uint,
	) (pending.PreConfirmedUpdate, error)
}

// Override BlockPreLatest to simulate errors and/or injected responses
func (m *MockDataSource) BlockPreLatest(ctx context.Context) (pending.PreLatest, error) {
	n := m.numCallsPending.Add(1)
	if uint(n) <= m.pendingErrorThreshold {
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
) (pending.PreConfirmedUpdate, error) {
	n := m.numCallsPreConfirmed.Add(1)
	if uint(n) <= m.preConfirmedErrorThreshold {
		return pending.PreConfirmedUpdate{}, errors.New("some error")
	}

	// fallback to embedded real method if no override set
	if m.PreConfirmedFunc != nil {
		return m.PreConfirmedFunc(
			ctx,
			number,
			blockIdentifier,
			knownTransactionCount,
			uint(n),
		)
	}
	preConfirmed := makeTestPreConfirmed(number)
	preConfirmed.Block.TransactionCount = number%10 + uint64(n)/2
	preConfirmed.BlockIdentifier = "mock"
	return pending.PreConfirmedUpdate{
		Mode:            pending.PreConfirmedFull,
		BlockIdentifier: preConfirmed.BlockIdentifier,
		FullBlock:       &preConfirmed,
	}, nil
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
			require.GreaterOrEqual(t, mockDS.numCallsPending.Load(), uint32(3),
				"expected at least 3 attempts (2 errors + 1 success)")
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
		out := make(chan *pending.PreConfirmedUpdate, 1)
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
		case update := <-out:
			require.NotNil(t, update)
			require.Equal(t, uint64(1), update.FullBlock.Block.Number)
			require.GreaterOrEqual(
				t,
				mockDS.numCallsPreConfirmed.Load(),
				uint32(3),
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

		out := make(chan *pending.PreConfirmedUpdate, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, out)
		})
		wg.Wait()

		require.GreaterOrEqual(t, mockDS.numCallsPreConfirmed.Load(), uint32(1),
			"Should have retried at least once before context cancelled")
	})

	t.Run("External trigger fires a fetch before the ticker would", func(t *testing.T) {
		// Ticker interval is set very high so any fetch we observe must have been
		// driven by the trigger channel rather than by the periodic ticker.
		mockDS := &MockDataSource{DataSource: dataSource}
		s := New(bc, mockDS, logger, 0, time.Hour, false, testDB)
		s.highestBlockHeader.Store(head0.Header)

		var preConfirmedBlockNumberToPoll atomic.Uint64
		preConfirmedBlockNumberToPoll.Store(1)
		out := make(chan *pending.PreConfirmedUpdate, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, out)
		})
		// LIFO: cancel before wg.Wait.
		defer wg.Wait()
		defer cancel()

		// Triggering a refresh should produce a fetch well within the ticker window.
		s.requestPreConfirmedRefresh()
		select {
		case update := <-out:
			require.NotNil(t, update)
			require.NotNil(t, update.FullBlock)
			require.Equal(t, uint64(1), update.FullBlock.Block.Number)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("trigger did not produce a fetch before the ticker")
		}
	})

	t.Run("Trigger storm collapses to one fetch while a fetch is running", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Block the data source until released so the first fetch stays running
			// while we dispatch the trigger storm. Any extra fetches would mean the
			// running guard failed to drop the storm requests.
			release := make(chan struct{})
			mockDS := &MockDataSource{
				DataSource: dataSource,
				PreConfirmedFunc: func(
					ctx context.Context,
					number uint64,
					_ string,
					_ uint64,
					_ uint,
				) (pending.PreConfirmedUpdate, error) {
					select {
					case <-release:
					case <-ctx.Done():
						return pending.PreConfirmedUpdate{}, ctx.Err()
					}
					preConf := makeTestPreConfirmed(number)
					preConf.BlockIdentifier = "mock"
					return pending.PreConfirmedUpdate{
						Mode:            pending.PreConfirmedFull,
						BlockIdentifier: preConf.BlockIdentifier,
						FullBlock:       &preConf,
					}, nil
				},
			}
			// Ticker interval set very high so the only thing that can drive a
			// fetch is our explicit trigger calls (synctest can advance virtual
			// time during Wait, so a small ticker would race with us).
			s := New(bc, mockDS, logger, 0, time.Hour, false, testDB)
			s.highestBlockHeader.Store(head0.Header)

			var preConfirmedBlockNumberToPoll atomic.Uint64
			preConfirmedBlockNumberToPoll.Store(1)
			out := make(chan *pending.PreConfirmedUpdate, 32)

			ctx, cancel := context.WithCancel(t.Context())
			var wg stdsync.WaitGroup
			wg.Go(func() {
				s.pollPreConfirmed(ctx, &preConfirmedBlockNumberToPoll, out)
			})
			defer wg.Wait()
			defer cancel()

			// Kick off the first fetch. synctest.Wait blocks until every other
			// goroutine in the bubble is durably blocked, which here means the
			// polling goroutine is parked inside the mock waiting on release.
			s.requestPreConfirmedRefresh()
			synctest.Wait()
			require.Equal(t, uint32(1), mockDS.numCallsPreConfirmed.Load(),
				"expected the first fetch to be running")

			// Storm of requests while the fetch is running: the running guard
			// must drop every one of them.
			for range 100 {
				s.requestPreConfirmedRefresh()
			}
			synctest.Wait()
			require.Equal(t, uint32(1), mockDS.numCallsPreConfirmed.Load(),
				"storm requests must be dropped by the running guard")

			// Release the fetch, let the goroutine deliver the update and go
			// back to waiting, then assert no extra fetch was issued.
			close(release)
			synctest.Wait()
			select {
			case <-out:
			default:
				t.Fatal("expected the first fetch to deliver a result")
			}
			require.Equal(t, uint32(1), mockDS.numCallsPreConfirmed.Load(),
				"trigger storm during fetch should not produce additional fetches")
		})
	})
}

//nolint:gocyclo // Convering multiple complex cases
func TestPollPendingData(t *testing.T) {
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

	fetchBlock := func(number uint64) (*core.Block, *core.StateUpdate) {
		block, err := gw.BlockByNumber(t.Context(), number)
		require.NoError(t, err)
		stateUpdate, err := gw.StateUpdate(t.Context(), number)
		require.NoError(t, err)
		return block, stateUpdate
	}

	t.Run("assert preConfirmed increment after preLatest arrival", func(t *testing.T) {
		block0, stateUpdate0 := fetchBlock(0)
		require.NoError(
			t, bc.Store(
				block0,
				&core.BlockCommitments{},
				stateUpdate0,
				map[felt.Felt]core.ClassDefinition{},
			),
		)

		// Mock data source to delay pre_latest (pending) while allowing pre_confirmed to arrive
		pendingFunc := func(context.Context) (pending.PreLatest, error) {
			head, err := bc.HeadsHeader()
			require.NoError(t, err)
			return makeEmptyPreLatestForParent(head), nil
		}

		mockDataSource := &MockDataSource{
			DataSource:            dataSource,
			PendingFunc:           pendingFunc,
			pendingErrorThreshold: 2, // introduce delay to receive pre_latest
		}
		s := New(bc, mockDataSource, logger, 50*time.Millisecond, 50*time.Millisecond, false, testDB)
		s.highestBlockHeader.Store(block0.Header)

		// Subscribe to pre-confirmed feed to observe stored pre_confirmed
		sub := s.preConfirmedDataFeed.SubscribeKeepLast()
		defer sub.Unsubscribe()

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		var wg stdsync.WaitGroup
		wg.Go(func() {
			s.pollPendingData(ctx)
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

		for {
			// After pending eventually succeeds, pre_latest should raise
			// target to 2; expect pre_confirmed for 2
			select {
			case pd := <-sub.Recv():
				require.NotNil(t, pd)
				if pd.GetBlock().Number == uint64(2) {
					return
				}

				if pd.GetBlock().Number > uint64(2) {
					t.Fatal("skipped pre_confirmed for number 2 and advanced further")
				}

			case <-ctx.Done():
				t.Fatal("did not broadcast pre_confirmed for number 2")
			}
		}
	})

	t.Run("assert delta updates are applied", func(t *testing.T) {
		testDB = memory.New()
		bc = blockchain.New(
			testDB,
			&networks.Sepolia,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)
		dataSource = NewFeederGatewayDataSource(bc, gw)
		// blocks must be fetched outside the synctest goroutine
		block0, stateUpdate0 := fetchBlock(0)
		block1, stateUpdate1 := fetchBlock(1)

		preLPoll := time.Second            // defaultPreLatestPollInterval
		preCPoll := 500 * time.Millisecond // defaultPreConfirmedPollInterval

		synctest.Test(t, func(t *testing.T) {
			// no PreLatest, so that we can focus on pre_confirmed polling.
			pendingFunc := func(context.Context) (pending.PreLatest, error) {
				return pending.PreLatest{}, errors.New("no PreLatest available")
			}
			preConfirmedFunc := func(
				ctx context.Context,
				number uint64,
				blockIdentifier string,
				knownTransactionCount uint64,
				numCalls uint,
			) (pending.PreConfirmedUpdate, error) {
				newTx := func() core.Transaction {
					return &core.InvokeTransaction{
						TransactionHash: felt.NewRandom[felt.Felt](),
					}
				}

				var response pending.PreConfirmedUpdate
				switch number {
				case 1:
					// block 1 will be polled 2 times in our test.
					// First time, return full block. Second time, return no change.
					switch numCalls {
					case 1:
						preConf := makeTestPreConfirmed(number)
						response = pending.PreConfirmedUpdate{
							Mode:            pending.PreConfirmedFull,
							BlockIdentifier: blockIdentifier,
							FullBlock:       &preConf,
						}
						response.FullBlock.Block.TransactionCount = number%10 + uint64(numCalls)/2
						response.FullBlock.BlockIdentifier = blockIdentifier
					case 2:
						response = pending.PreConfirmedUpdate{
							Mode:            pending.PreConfirmedNoChange,
							BlockIdentifier: blockIdentifier,
						}
					}
				case 2:
					// block 2 will be polled 4 times in our test.
					// Full block > Delta Change > New Identifier > No change.
					switch numCalls {
					case 3:
						preConf := makeTestPreConfirmed(number)
						response = pending.PreConfirmedUpdate{
							Mode:            pending.PreConfirmedFull,
							BlockIdentifier: blockIdentifier,
							FullBlock:       &preConf,
						}
						response.FullBlock.Block.TransactionCount = number%10 + uint64(numCalls)/2
						response.FullBlock.BlockIdentifier = blockIdentifier
					case 4:
						response = pending.PreConfirmedUpdate{
							Mode:               pending.PreConfirmedDelta,
							BlockIdentifier:    blockIdentifier,
							AppendTransactions: []core.Transaction{newTx()},
							AppendReceipts:     []*core.TransactionReceipt{{}},
							AppendStateDiffs:   []*core.StateDiff{{}},
						}
					case 5:
						preConf := makeTestPreConfirmed(number)
						response = pending.PreConfirmedUpdate{
							Mode:            pending.PreConfirmedFull,
							BlockIdentifier: "0xdeadbeef",
							FullBlock:       &preConf,
						}
						response.FullBlock.Block.TransactionCount = knownTransactionCount
						response.FullBlock.BlockIdentifier = "0xdeadbeef"
					case 6:
						response = pending.PreConfirmedUpdate{
							Mode:            pending.PreConfirmedNoChange,
							BlockIdentifier: blockIdentifier,
						}
					}
				default:
					return pending.PreConfirmedUpdate{}, fmt.Errorf("unexpected number: %d", number)
				}

				return response, nil
			}

			mockDataSource := &MockDataSource{
				DataSource:       dataSource,
				PendingFunc:      pendingFunc,
				PreConfirmedFunc: preConfirmedFunc,
			}

			s := New(
				bc,
				mockDataSource,
				logger,
				preLPoll,
				preCPoll,
				false,
				testDB,
			)

			// Subscribe to pre-confirmed feed to observe stored pre_confirmed
			sub := s.preConfirmedDataFeed.SubscribeKeepLast()
			defer sub.Unsubscribe()

			go func() {
				s.pollPendingData(t.Context())
			}()
			synctest.Wait()

			// store block0; sets preConfirmed target to 1
			require.NoError(
				t, bc.Store(
					block0,
					&core.BlockCommitments{},
					stateUpdate0,
					map[felt.Felt]core.ClassDefinition{},
				),
			)
			s.highestBlockHeader.Store(block0.Header)
			s.newHeads.Send(block0)
			synctest.Wait()

			// 1st preConfirmed tick; expected pre_confirmed = 1.
			// Full block is returned
			time.Sleep(preCPoll)
			synctest.Wait()

			pc := <-sub.Recv()
			require.NotNil(t, pc)
			assert.Equal(t, uint64(1), pc.Block.Number)
			synctest.Wait()

			// 2nd preConfirmed tick; preConfirmed target is still 1
			// No change in preConfirmed
			time.Sleep(preCPoll)
			synctest.Wait()
			select {
			case <-sub.Recv():
				t.Fatal("'no change' must not trigger a new PreConfirmed broadcast")
			default:
			}

			// store block1; sets preConfirmed target to 2
			require.NoError(
				t, bc.Store(
					block1,
					&core.BlockCommitments{},
					stateUpdate1,
					map[felt.Felt]core.ClassDefinition{},
				),
			)
			s.highestBlockHeader.Store(block1.Header)
			s.newHeads.Send(block1)
			synctest.Wait()

			// 3rd preConfirmed tick; expected pre_confirmed = 2.
			// Full block is returned
			time.Sleep(preCPoll)
			synctest.Wait()

			pc = <-sub.Recv()
			require.NotNil(t, pc)
			assert.Equal(t, uint64(2), pc.Block.Number)
			synctest.Wait()

			// 4th preConfirmed tick; pre_confirmed target is still 2.
			// PreConfirmed 2 arrives with delta update.
			time.Sleep(preCPoll)
			synctest.Wait()

			deltaPc := <-sub.Recv()
			require.NotNil(t, deltaPc)
			assert.Equal(t, pc.Block.Number, deltaPc.Block.Number)
			assert.NotEqual(t, pc, deltaPc)
			synctest.Wait()

			// 5th preConfirmed tick; pre_confirmed target is still 2.
			// PreConfirmed 2 arrives with new identifier. Node must discard current
			// preConfirmed and store the new one.
			time.Sleep(preCPoll)
			synctest.Wait()

			pc = <-sub.Recv()
			require.NotNil(t, pc)
			assert.Equal(t, deltaPc.Block.Number, pc.Block.Number)
			assert.NotEqual(t, deltaPc.BlockIdentifier, pc.BlockIdentifier)
			synctest.Wait()

			// 6th preConfirmed tick; preConfirmed target is still 2
			// No change in preConfirmed
			time.Sleep(preCPoll)
			synctest.Wait()
			select {
			case <-sub.Recv():
				t.Fatal("'no change' must not trigger a new PreConfirmed broadcast")
			default:
			}
		})
	})
}

func TestStorePreConfirmed(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(
		testDB,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	logger := log.NewNopZapLogger()
	client := feeder.NewTestClient(t, &networks.Mainnet)
	gw := adaptfeeder.New(client)

	s := New(bc, NewFeederGatewayDataSource(bc, nil), logger, 0, 0, false, testDB)

	t.Run("stores pre_confirmed when there is none (first entry)", func(t *testing.T) {
		preConfirmed := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{Number: 0},
			},
			StateUpdate: &core.StateUpdate{},
		}
		t.Run("head is nil", func(t *testing.T) {
			written, err := s.StorePreConfirmed(&preConfirmed)
			require.NoError(t, err)
			require.True(t, written)
			ptr := s.preConfirmed.Load()
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
			map[felt.Felt]core.ClassDefinition{},
		))
		t.Run("not valid for head", func(t *testing.T) {
			s.preConfirmed.Store(nil)
			written, err := s.StorePreConfirmed(&preConfirmed)
			require.Error(t, err)
			require.False(t, written)
		})
	})

	t.Run("returns error if ProtocolVersion unsupported", func(t *testing.T) {
		pc := &pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:          1,
					ProtocolVersion: core.LatestVer.IncMajor().String(),
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
		invalidPreConfirmed := pending.PreConfirmed{
			Block:       &core.Block{Header: &core.Header{Number: 0}},
			StateUpdate: &core.StateUpdate{},
		}
		// Insert invalid pending (simulate old data)
		s.preConfirmed.Store(&invalidPreConfirmed)
		pc := &pending.PreConfirmed{
			Block:       &core.Block{Header: &core.Header{Number: head.Number + 1}},
			StateUpdate: &core.StateUpdate{},
		}
		written, err := s.StorePreConfirmed(pc)
		require.NoError(t, err)
		require.True(t, written)
	})

	t.Run("ignores pre_confirmed with fewer or equal txs for the same block number (but updates attachment)", func(t *testing.T) {
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		// Store "better" with higher tx count
		better := &pending.PreConfirmed{
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

		// Attempt to store "worse" but with a pre_latest attachment;
		// should keep existing but update attachment
		pl := makeEmptyPreLatestForParent(head)

		worse := &pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 1,
				},
			},
			PreLatest:   &pl,
			StateUpdate: &core.StateUpdate{},
		}
		written, err = s.StorePreConfirmed(worse)
		require.NoError(t, err)
		require.False(t, written)

		ptr := s.preConfirmed.Load()
		require.NotNil(t, ptr)
		stored := *ptr
		require.NotNil(t, stored.PreLatest, "attachment should be updated even if not swapping blocks")
		require.Equal(t, &pl, stored.PreLatest, "attachment should match incoming")
	})

	t.Run("accepts pre_confirmed with more txs for same block number", func(t *testing.T) {
		s.preConfirmed.Store(nil)
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		worse := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 1,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err := s.StorePreConfirmed(&worse)
		require.NoError(t, err)
		require.True(t, written)
		ptr := s.preConfirmed.Load()
		require.Equal(t, worse, *ptr)

		better := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 2,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err = s.StorePreConfirmed(&better)
		require.NoError(t, err)
		require.True(t, written)
		ptr = s.preConfirmed.Load()
		require.Equal(t, better, *ptr)
	})

	t.Run("accepts more recent pre_confirmed regardless tx count", func(t *testing.T) {
		s.preConfirmed.Store(nil)
		head, err := bc.HeadsHeader()
		require.NoError(t, err)

		old := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 5,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err := s.StorePreConfirmed(&old)
		require.NoError(t, err)
		require.True(t, written)
		ptr := s.preConfirmed.Load()
		require.Equal(t, old, *ptr)

		// Attach prelatest to make validate pass for pre_confirmed number == head + 2.
		// Similar as storing head + 1
		preLatest := &pending.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: head.Hash,
					Number:     head.Number + 1,
				},
			},
		}
		newer := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 2,
					TransactionCount: 2,
				},
			},
			PreLatest:   preLatest,
			StateUpdate: &core.StateUpdate{},
		}
		written, err = s.StorePreConfirmed(&newer)
		require.NoError(t, err)
		require.True(t, written)
		ptr = s.preConfirmed.Load()
		require.Equal(t, newer, *ptr)
	})

	t.Run("ignores valid older pre_confirmed", func(t *testing.T) {
		// A valid older pre_confirmed value occurs when the head is at N;
		// - pre_confirmed at N + 1
		// - pre_confirmed is at N + 2 (with pre-latest).
		// However N+1 must not overwrite N+2.
		s.preConfirmed.Store(nil)
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		// Attach prelatest to make validate pass for pre_confirmed number == head + 2.
		// Similar as storing head + 1
		preLatest := &pending.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: head.Hash,
					Number:     head.Number + 1,
				},
			},
		}
		newer := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 2,
					TransactionCount: 2,
				},
			},
			PreLatest:   preLatest,
			StateUpdate: &core.StateUpdate{},
		}
		written, err := s.StorePreConfirmed(&newer)
		require.NoError(t, err)
		require.True(t, written)
		ptr := s.preConfirmed.Load()
		require.Equal(t, newer, *ptr)
		// Valid older pre_confirmed
		old := pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           head.Number + 1,
					TransactionCount: 5,
				},
			},
			StateUpdate: &core.StateUpdate{},
		}
		written, err = s.StorePreConfirmed(&old)
		require.NoError(t, err)
		require.False(t, written)
		ptr = s.preConfirmed.Load()
		require.Equal(t, newer, *ptr)
	})
}

func makeTestPreConfirmed(num uint64) pending.PreConfirmed {
	receipts := make([]*core.TransactionReceipt, 0)
	preConfirmedBlock := &core.Block{
		// pre_confirmed block does not have parent hash
		Header: &core.Header{
			SequencerAddress: feltOne,
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
	preConfirmed := pending.PreConfirmed{
		Block: preConfirmedBlock,
		StateUpdate: &core.StateUpdate{
			StateDiff: &stateDiff,
		},
		NewClasses:            make(map[felt.Felt]core.ClassDefinition, 0),
		TransactionStateDiffs: make([]*core.StateDiff, 0),
		CandidateTxs:          make([]core.Transaction, 0),
	}
	return preConfirmed
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

// TestPreConfirmedUpdateFrequency prints metrics comparing the ticker-only
// baseline (pre-PR behaviour) with the request-driven path. The baseline
// subtest never fires triggers; the request-driven subtest fires them at a
// fixed rate. Run: go test -v -run TestPreConfirmedUpdateFrequency ./sync/
func TestPreConfirmedUpdateFrequency(t *testing.T) {
	testDB := memory.New()
	bc := blockchain.New(testDB, &networks.Sepolia,
		blockchain.WithNewState(statetestutils.UseNewState()))
	client := feeder.NewTestClient(t, &networks.Sepolia)
	gw := adaptfeeder.New(client)
	dataSource := NewFeederGatewayDataSource(bc, gw)

	head0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)
	su0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)
	require.NoError(t, bc.Store(head0, &core.BlockCommitments{}, su0,
		map[felt.Felt]core.ClassDefinition{}))

	const (
		tickerInterval = 500 * time.Millisecond // default --preconfirmed-poll-interval
		runDuration    = 5 * time.Second
		triggerRate    = 50 * time.Millisecond // simulated RPC traffic at 20 Hz
	)

	type scenario struct {
		name            string
		triggerInterval time.Duration // 0 means no triggers (ticker-only baseline)
	}
	scenarios := []scenario{
		{"baseline_ticker_only", 0},
		{"request_driven_20Hz", triggerRate},
	}

	// Run each scenario across a few realistic feeder latencies.
	latencies := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}

	for _, fetchLatency := range latencies {
		for _, sc := range scenarios {
			name := fmt.Sprintf("%s/fetch=%s", sc.name, fetchLatency)
			t.Run(name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					mockDS := &MockDataSource{
						DataSource: dataSource,
						PreConfirmedFunc: func(
							ctx context.Context,
							number uint64,
							_ string, _ uint64, _ uint,
						) (pending.PreConfirmedUpdate, error) {
							select {
							case <-time.After(fetchLatency):
							case <-ctx.Done():
								return pending.PreConfirmedUpdate{}, ctx.Err()
							}
							preConf := makeTestPreConfirmed(number)
							preConf.BlockIdentifier = "mock"
							return pending.PreConfirmedUpdate{
								Mode:            pending.PreConfirmedFull,
								BlockIdentifier: preConf.BlockIdentifier,
								FullBlock:       &preConf,
							}, nil
						},
					}
					syn := New(bc, mockDS, log.NewNopZapLogger(), 0, tickerInterval, false, testDB)
					syn.highestBlockHeader.Store(head0.Header)

					var target atomic.Uint64
					target.Store(1)
					out := make(chan *pending.PreConfirmedUpdate, 1024)

					ctx, cancel := context.WithCancel(t.Context())

					go syn.pollPreConfirmed(ctx, &target, out)

					var emitted atomic.Uint32
					if sc.triggerInterval > 0 {
						go func() {
							ticker := time.NewTicker(sc.triggerInterval)
							defer ticker.Stop()
							for {
								select {
								case <-ctx.Done():
									return
								case <-ticker.C:
									emitted.Add(1)
									syn.requestPreConfirmedRefresh()
								}
							}
						}()
					}

					var intervals []time.Duration
					var lastUpdate time.Time
					recorderDone := make(chan struct{})
					go func() {
						defer close(recorderDone)
						for {
							select {
							case <-ctx.Done():
								return
							case <-out:
								now := time.Now()
								if !lastUpdate.IsZero() {
									intervals = append(intervals, now.Sub(lastUpdate))
								}
								lastUpdate = now
							}
						}
					}()

					time.Sleep(runDuration)
					cancel()
					<-recorderDone

					fetches := mockDS.numCallsPreConfirmed.Load()
					trigs := emitted.Load()
					var dropped uint32
					if trigs >= fetches {
						dropped = trigs - fetches
					}
					updates := uint32(len(intervals) + 1)
					if lastUpdate.IsZero() {
						updates = 0
					}
					t.Logf(
						"%s fetch=%s: updates=%d (%.1f/s) fetches=%d "+
							"triggers_emitted=%d triggers_dropped=%d "+
							"mean_interval=%v p99_interval=%v",
						sc.name, fetchLatency,
						updates, float64(updates)/runDuration.Seconds(),
						fetches, trigs, dropped,
						meanDuration(intervals), p99Duration(intervals),
					)
				})
			})
		}
	}
}

func meanDuration(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}

func p99Duration(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	slices.Sort(sorted)
	idx := int(float64(len(sorted)) * 0.99)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
