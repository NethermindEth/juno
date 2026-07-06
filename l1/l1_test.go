package l1_test

import (
	"context"
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFailToCreateSubscription(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")

	network := networks.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)

	provider.
		EXPECT().
		WatchStateUpdate(gomock.Any(), gomock.Any()).
		Return(newFakeSubscription(), err).
		AnyTimes()

	provider.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.L1ChainID, nil).
		Times(1)

	// catchUp runs before subscribe; let it complete cleanly so the test
	// reaches the subscription failure path it actually exercises.
	provider.EXPECT().LatestHeight(gomock.Any()).Return(uint64(0), nil).AnyTimes()
	provider.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(0), nil).AnyTimes()
	provider.
		EXPECT().
		FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	provider.EXPECT().Close().Times(1)

	client := l1.NewClient(
		provider,
		chain,
		nopLog,
		l1.WithResubscribeDelay(0),
		l1.WithPollFinalisedInterval(time.Nanosecond),
	)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	// Resubscribe loop keeps failing; ctx timeout is treated as a clean
	// shutdown, so Run returns nil rather than surfacing the cancellation.
	require.NoError(t, client.Run(ctx))
	cancel()
}

func TestMismatchedChainID(t *testing.T) {
	t.Parallel()

	network := networks.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)

	provider.EXPECT().Close().Times(1)
	provider.
		EXPECT().
		ChainID(gomock.Any()).
		Return(new(big.Int), nil).
		Times(1)

	client := l1.NewClient(
		provider,
		chain,
		nopLog,
		l1.WithResubscribeDelay(0),
		l1.WithPollFinalisedInterval(time.Nanosecond),
	)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	err := client.Run(ctx)
	require.ErrorContains(t, err, "mismatched network id between L1 and L2")
	require.ErrorContains(t, err, "--eth-node")
}

// TestChainIDCheckTimeout asserts a chain-ID probe gives up after 30s with a
// user-actionable error when the L1 endpoint never answers eth_chainId. It uses
// the one-shot CatchUpL1Head path, which fails fast (Run now retries instead,
// per issue #1385). synctest advances the 30s wait in virtual time.
func TestChainIDCheckTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := networks.Mainnet
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		provider := mocks.NewMockL1StateProvider(ctrl)
		provider.EXPECT().Close().Times(1)
		provider.
			EXPECT().
			ChainID(gomock.Any()).
			DoAndReturn(func(ctx context.Context) (*big.Int, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}).
			Times(1)

		client := l1.NewClient(provider, chain, nopLog)

		err := client.CatchUpL1Head(t.Context())
		require.ErrorContains(t, err, "eth_chainId did not respond within")
		require.ErrorContains(t, err, "--eth-node")
	})
}

// TestChainIDFetchError asserts a non-timeout eth_chainId failure is wrapped and
// surfaced by the fail-fast CatchUpL1Head path (Run retries it instead, #1385).
func TestChainIDFetchError(t *testing.T) {
	t.Parallel()

	network := networks.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)
	provider.EXPECT().Close().Times(1)
	rpcErr := errors.New("boom")
	provider.
		EXPECT().
		ChainID(gomock.Any()).
		Return(nil, rpcErr).
		Times(1)

	client := l1.NewClient(provider, chain, nopLog)

	err := client.CatchUpL1Head(t.Context())
	require.ErrorContains(t, err, "retrieving Ethereum chain ID")
	require.ErrorIs(t, err, rpcErr)
}

// TestTransientChainIDErrorDoesNotShutDownNode is the regression guard for issue
// #1385: a transient eth_chainId failure (the rate-limit error from the issue) is
// retried, not fatal. ChainID keeps failing; the node-wide context is cancelled
// on the third attempt. Run must then return no error, having retried more than
// once instead of aborting on the first failure (the old, node-killing behaviour).
func TestTransientChainIDErrorDoesNotShutDownNode(t *testing.T) {
	t.Parallel()

	network := networks.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	provider := mocks.NewMockL1StateProvider(ctrl)
	provider.EXPECT().Close().Times(1)

	const cancelAfter = 3
	var chainIDCalls atomic.Int32
	rateLimitErr := errors.New("daily request count exceeded, request rate limited")
	provider.
		EXPECT().
		ChainID(gomock.Any()).
		DoAndReturn(func(context.Context) (*big.Int, error) {
			if chainIDCalls.Add(1) == cancelAfter {
				cancel() // shut down while L1 is still retrying
			}
			return nil, rateLimitErr
		}).
		MinTimes(cancelAfter)

	// Once ctx is cancelled, Run returns early after ensureChainID without
	// entering catch-up or the watch loop, so no other L1StateProvider calls occur.

	client := l1.NewClient(
		provider, chain, nopLog,
		l1.WithResubscribeDelay(0),
		l1.WithPollFinalisedInterval(time.Nanosecond),
	)

	require.NoError(t, client.Run(ctx))
	require.GreaterOrEqual(t, chainIDCalls.Load(), int32(cancelAfter),
		"a transient chain ID error should be retried, not shut the node down")
}

// TestFinalisedHeightTimeoutDuringCatchUp asserts that the L1 catch-up startup
// scan gives up on a hung eth_getBlockByNumber("finalized") call with a
// user-actionable error pointing at --eth-node, instead of stalling forever.
// Runs inside a synctest bubble so the 30s per-RPC timeout advances in virtual
// time. CatchUpL1Head (rather than Run) is exercised because Run swallows
// catch-up errors as best-effort, whereas CatchUpL1Head propagates them.
func TestFinalisedHeightTimeoutDuringCatchUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := networks.Mainnet
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		provider := mocks.NewMockL1StateProvider(ctrl)
		provider.EXPECT().Close().Times(1)
		provider.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
		provider.EXPECT().LatestHeight(gomock.Any()).Return(uint64(1000), nil).Times(1)
		provider.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			DoAndReturn(func(ctx context.Context) (uint64, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			}).
			Times(1)

		err := l1.NewClient(provider, chain, nopLog).CatchUpL1Head(t.Context())
		require.ErrorContains(t, err, `eth_getBlockByNumber("finalized")`)
		require.ErrorContains(t, err, "did not respond within")
		require.ErrorContains(t, err, "--eth-node")
	})
}

// TestLatestHeightTimeoutDuringCatchUp covers the first RPC in the catch-up
// scan. Same shape as TestFinalisedHeightTimeoutDuringCatchUp but for
// eth_blockNumber instead of eth_getBlockByNumber("finalized").
func TestLatestHeightTimeoutDuringCatchUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := networks.Mainnet
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		provider := mocks.NewMockL1StateProvider(ctrl)
		provider.EXPECT().Close().Times(1)
		provider.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
		provider.
			EXPECT().
			LatestHeight(gomock.Any()).
			DoAndReturn(func(ctx context.Context) (uint64, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			}).
			Times(1)

		err := l1.NewClient(provider, chain, nopLog).CatchUpL1Head(t.Context())
		require.ErrorContains(t, err, "eth_blockNumber did not respond within")
		require.ErrorContains(t, err, "--eth-node")
	})
}

// TestFilterStateUpdateTimeoutDuringCatchUp covers the eth_getLogs path. It
// has a longer (60s) production timeout than the two height calls; the test
// just relies on synctest to fast-forward whatever the timeout happens to be.
func TestFilterStateUpdateTimeoutDuringCatchUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := networks.Mainnet
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		provider := mocks.NewMockL1StateProvider(ctrl)
		provider.EXPECT().Close().Times(1)
		provider.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
		provider.EXPECT().LatestHeight(gomock.Any()).Return(uint64(1000), nil).Times(1)
		provider.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(500), nil).Times(1)
		provider.
			EXPECT().
			FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _, _ uint64) ([]*l1.StateUpdate, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}).
			Times(1)

		err := l1.NewClient(provider, chain, nopLog).CatchUpL1Head(t.Context())
		require.ErrorContains(t, err, "eth_getLogs did not respond within")
		require.ErrorContains(t, err, "--eth-node")
	})
}

// TestFinalisedHeightRetryLoopProgressesPastHang asserts that the retry loop
// invoked from setL1Head no longer spins forever when FinalisedHeight hangs:
// the per-call timeout fires, the loop sleeps for resubscribeDelay, retries,
// and a subsequent successful response is used to write the L1 head.
//
// Drives through the public CatchUpL1Head path. Mocks are sequenced so that
// the FinalisedHeight call inside the catch-up scan succeeds, then the first
// call from the retry loop hangs, then the second call from the retry loop
// succeeds. Asserts via chain.L1Head() that the head was eventually written.
//
// Note: without the per-call timeout, this test would deadlock under synctest
// (no timer to advance), so a passing run is itself proof the timeout is wired.
func TestFinalisedHeightRetryLoopProgressesPastHang(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := networks.Mainnet
		ctrl := gomock.NewController(t)
		nopLog := log.NewNopZapLogger()
		chain := blockchain.New(
			memory.New(),
			&network,
			blockchain.WithNewState(statetestutils.UseNewState()),
		)

		provider := mocks.NewMockL1StateProvider(ctrl)
		provider.EXPECT().Close().Times(1)
		provider.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).Times(1)
		provider.EXPECT().LatestHeight(gomock.Any()).Return(uint64(10), nil).Times(1)

		// FinalisedHeight is called three times in this flow:
		//   1) inside catchUpL1HeadUpdates — needs a real value so foundFinalised
		//      becomes true and the scan exits into setL1Head;
		//   2) first iteration of setL1Head's retry loop — hangs, exercising the
		//      new per-call timeout;
		//   3) second iteration — succeeds, the loop returns and the head is set.
		catchupCall := provider.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			Return(uint64(5), nil).
			Times(1)
		hungCall := provider.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			DoAndReturn(func(ctx context.Context) (uint64, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			}).
			Times(1).
			After(catchupCall)
		provider.
			EXPECT().
			FinalisedHeight(gomock.Any()).
			Return(uint64(5), nil).
			Times(1).
			After(hungCall)

		// One finalised event at L1=3 (≤ finalised=5) so foundFinalised flips
		// and the catch-up loop reaches setL1Head with something to commit.
		event := &l1.StateUpdate{
			L2BlockNumber: 7,
			L2BlockHash:   new(felt.Felt).SetUint64(7),
			StateRoot:     new(felt.Felt).SetUint64(7),
			L1RefHeight:   3,
		}
		provider.
			EXPECT().
			FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*l1.StateUpdate{event}, nil).
			Times(1)

		client := l1.NewClient(provider, chain, nopLog, l1.WithResubscribeDelay(time.Second))
		require.NoError(t, client.CatchUpL1Head(t.Context()))

		got, err := chain.L1Head()
		require.NoError(t, err)
		require.Equal(t, core.L1Head{
			BlockNumber: 7,
			BlockHash:   new(felt.Felt).SetUint64(7),
			StateRoot:   new(felt.Felt).SetUint64(7),
		}, got)
	})
}

func TestEventListener(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)
	provider.
		EXPECT().
		WatchStateUpdate(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, sink chan<- *l1.StateUpdate) {
			sink <- &l1.StateUpdate{
				L2BlockHash: new(felt.Felt),
				StateRoot:   new(felt.Felt),
			}
		}).
		Return(newFakeSubscription(), nil).
		Times(1)

	provider.
		EXPECT().
		FinalisedHeight(gomock.Any()).
		Return(uint64(0), nil).
		AnyTimes()

	provider.
		EXPECT().
		LatestHeight(gomock.Any()).
		Return(uint64(0), nil).
		AnyTimes()

	provider.
		EXPECT().
		FilterStateUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	provider.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.L1ChainID, nil).
		Times(1)

	provider.EXPECT().Close().Times(1)

	var got *core.L1Head
	client := l1.NewClient(
		provider, chain, nopLog,
		l1.WithResubscribeDelay(0),
		l1.WithPollFinalisedInterval(time.Nanosecond),
		l1.WithEventListener(l1.SelectiveListener{
			OnNewL1HeadCb: func(head *core.L1Head) {
				got = head
			},
		}),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	require.Equal(t, &core.L1Head{
		BlockHash: new(felt.Felt),
		StateRoot: new(felt.Felt),
	}, got)
}

func TestEventListenerCatchUp(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)
	provider.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.L1ChainID, nil).
		Times(1)

	// Live subscription delivers nothing; the catch-up scan alone must
	// populate nonFinalisedLogs so setL1Head fires the listener callback.
	provider.
		EXPECT().
		WatchStateUpdate(gomock.Any(), gomock.Any()).
		Return(newFakeSubscription(), nil).
		AnyTimes()

	// LatestHeight=10, FinalisedHeight=5, catchUpChunkSize=1000 → single chunk [0, 10].
	provider.EXPECT().LatestHeight(gomock.Any()).Return(uint64(10), nil).Times(1)
	provider.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(5), nil).AnyTimes()

	backfilled := &l1.StateUpdate{
		L2BlockNumber: 7,
		L2BlockHash:   new(felt.Felt).SetUint64(7),
		StateRoot:     new(felt.Felt).SetUint64(7),
		L1RefHeight:   3,
	}
	provider.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(0), uint64(10)).
		Return([]*l1.StateUpdate{backfilled}, nil).
		Times(1)

	provider.EXPECT().Close().Times(1)

	var got *core.L1Head
	client := l1.NewClient(
		provider, chain, nopLog,
		l1.WithResubscribeDelay(0),
		l1.WithPollFinalisedInterval(time.Hour),
		l1.WithEventListener(l1.SelectiveListener{
			OnNewL1HeadCb: func(head *core.L1Head) {
				got = head
			},
		}),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	want := &core.L1Head{
		BlockNumber: 7,
		BlockHash:   new(felt.Felt).SetUint64(7),
		StateRoot:   new(felt.Felt).SetUint64(7),
	}
	require.Equal(t, want, got)

	persisted, err := chain.L1Head()
	require.NoError(t, err)
	require.Equal(t, *want, persisted)
}

// TestCatchUpL1Head is the regression for the history-pruning migration
// bootstrap (node/migration.go). The migration crashes with a bare "key
// not found" if it runs before any L1 head is on disk; the fix calls
// Client.CatchUpL1Head in node/migration.go to write one up front. This
// test pins the contract that CatchUpL1Head used by that path actually
// persists the head — if a future change drops the setL1Head call inside
// catchUpL1HeadUpdates, or short-circuits Client.CatchUpL1Head before it
// reaches catch-up, chain.L1Head() below fails and the bug returns.
func TestCatchUpL1Head(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)
	provider.EXPECT().ChainID(gomock.Any()).Return(network.L1ChainID, nil).AnyTimes()
	provider.EXPECT().LatestHeight(gomock.Any()).Return(uint64(10), nil).AnyTimes()
	provider.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(5), nil).AnyTimes()
	provider.
		EXPECT().
		FilterStateUpdate(gomock.Any(), uint64(0), uint64(10)).
		Return([]*l1.StateUpdate{{
			L2BlockNumber: 7,
			L2BlockHash:   new(felt.Felt).SetUint64(7),
			StateRoot:     new(felt.Felt).SetUint64(7),
			L1RefHeight:   3,
		}}, nil).
		AnyTimes()
	provider.EXPECT().Close().AnyTimes()

	client := l1.NewClient(provider, chain, nopLog)
	require.NoError(t, client.CatchUpL1Head(t.Context()))

	persisted, err := chain.L1Head()
	require.NoError(t, err)
	require.Equal(t, core.L1Head{
		BlockNumber: 7,
		BlockHash:   new(felt.Felt).SetUint64(7),
		StateRoot:   new(felt.Felt).SetUint64(7),
	}, persisted)
}

func TestCatchUpL1Head_ChainIDMismatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := log.NewNopZapLogger()
	network := networks.Mainnet
	chain := blockchain.New(
		memory.New(),
		&network,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	provider := mocks.NewMockL1StateProvider(ctrl)
	provider.EXPECT().ChainID(gomock.Any()).Return(big.NewInt(999), nil)
	provider.EXPECT().Close()

	err := l1.NewClient(provider, chain, nopLog).CatchUpL1Head(t.Context())
	require.ErrorContains(t, err, "mismatched network id between L1 and L2")
	require.ErrorContains(t, err, "--eth-node")
}
