package rpcv10

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func preConfirmedTestBlock(identifier string, hashes ...uint64) starknet.PreConfirmedBlock {
	block := starknet.PreConfirmedBlock{
		BlockIdentifier:  identifier,
		Version:          core.Ver0_14_0.String(),
		SequencerAddress: &felt.One,
		Timestamp:        1,
		L1GasPrice:       &starknet.GasPrice{PriceInWei: &felt.One, PriceInFri: &felt.One},
		L2GasPrice:       &starknet.GasPrice{PriceInWei: &felt.One, PriceInFri: &felt.One},
		L1DataGasPrice:   &starknet.GasPrice{PriceInWei: &felt.One, PriceInFri: &felt.One},
	}
	for _, hash := range hashes {
		hashFelt := felt.NewFromUint64[felt.Felt](hash)
		empty := []felt.Felt{}
		block.Transactions = append(block.Transactions, starknet.Transaction{
			Hash: hashFelt, Type: starknet.TxnInvoke, Version: &felt.One,
			CallData: &empty, Signature: &empty,
		})
		block.Receipts = append(block.Receipts, &starknet.TransactionReceipt{TransactionHash: hashFelt})
		block.TransactionStateDiffs = append(block.TransactionStateDiffs,
			&starknet.StateDiff{Nonces: map[string]*felt.Felt{felt.One.String(): hashFelt}})
	}
	return block
}

func preConfirmedTestDelta(identifier string, hashes ...uint64) starknet.PreConfirmedDeltaUpdate {
	block := preConfirmedTestBlock(identifier, hashes...)
	return starknet.PreConfirmedDeltaUpdate{
		BlockIdentifier:       identifier,
		Transactions:          block.Transactions,
		Receipts:              block.Receipts,
		TransactionStateDiffs: block.TransactionStateDiffs,
	}
}

type preConfirmedTestFixture struct {
	state        *mocks.MockStateReader
	handler      *Handler
	storage      *preconfirmed.ChainStorage
	oldest       atomic.Uint64
	baseHash     atomic.Pointer[felt.Felt]
	revealedHash atomic.Pointer[felt.Felt]
	opens        atomic.Uint64
	closes       atomic.Uint64
}

func newPreConfirmedTestFixture(t testing.TB, runner vm.VM) *preConfirmedTestFixture {
	t.Helper()
	f := &preConfirmedTestFixture{storage: preconfirmed.NewChainStorage()}
	f.oldest.Store(20)
	f.baseHash.Store(felt.NewFromUint64[felt.Felt](99))
	f.revealedHash.Store(felt.NewFromUint64[felt.Felt](80))
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	syncReader := mocks.NewMockSyncReader(ctrl)
	state := mocks.NewMockStateReader(ctrl)
	f.state = state
	state.EXPECT().ContractNonce(&felt.One).Return(felt.Zero, nil).AnyTimes()
	reader.EXPECT().BlockNumberAndIndexByTxHash(gomock.Any()).
		Return(uint64(0), uint64(0), db.ErrKeyNotFound).AnyTimes()
	reader.EXPECT().BlockHeaderHashByNumber(gomock.Any()).DoAndReturn(
		func(number uint64) (*felt.Felt, error) {
			if number == f.oldest.Load()-1 {
				return f.baseHash.Load(), nil
			}
			return f.revealedHash.Load(), nil
		}).AnyTimes()
	reader.EXPECT().StateAtBlockHash(gomock.Any()).DoAndReturn(
		func(hash *felt.Felt) (core.StateReader, func() error, error) {
			require.Equal(t, f.baseHash.Load(), hash)
			f.opens.Add(1)
			return state, func() error { f.closes.Add(1); return nil }, nil
		}).AnyTimes()
	syncReader.EXPECT().PreConfirmedChain().DoAndReturn(func() (preconfirmed.ChainReader, error) {
		return f.storage.SnapshotForBlock(f.oldest.Load()), nil
	}).AnyTimes()
	f.handler = New(reader, syncReader, runner, log.NewNopZapLogger())
	return f
}

func (f *preConfirmedTestFixture) apply(
	t testing.TB, update starknet.PreConfirmedUpdate, number, count uint64,
) {
	t.Helper()
	_, err := f.storage.ApplyUpdate(update, number, count, f.oldest.Load(), nil)
	require.NoError(t, err)
}

func (f *preConfirmedTestFixture) trace(ctx context.Context, hash uint64) preConfirmedTestResult {
	trace, header, err := f.handler.TraceTransaction(
		ctx, felt.NewFromUint64[felt.TransactionHash](hash),
	)
	return preConfirmedTestResult{trace, header, err}
}

func TestPreConfirmedTraceReuseAcrossUpdates(t *testing.T) {
	var executions atomic.Uint64
	runner := &progressiveTestVM{trace: func(
		txs []core.Transaction, state core.StateReader,
	) (vm.ExecutionResults, error) {
		executions.Add(1)
		nonce, err := state.ContractNonce(&felt.One)
		require.NoError(t, err)
		require.Equal(t, txs[0].Hash().Uint64()-1, nonce.Uint64(), "execute against the preceding state")
		result := progressiveTestResults(txs)
		result.Traces[0].Type = vm.TxnInvoke
		result.Traces[0].ExecuteInvocation = &vm.ExecuteInvocation{RevertReason: "reverted"}
		return result, nil
	}}
	f := newPreConfirmedTestFixture(t, runner)
	f.apply(t, preConfirmedTestBlock("round", 1), 20, 0)
	first := f.trace(t.Context(), 1)
	require.Nil(t, first.err)
	require.Equal(t, "1", first.header.Get(ExecutionStepsHeader))
	require.Equal(t, "reverted", first.trace.ExecuteInvocation.RevertReason)
	cache := f.handler.preConfirmedTraceCaches[20]
	f.apply(t, preConfirmedTestDelta("round", 2), 20, 1)
	f.handler.refreshPreConfirmedTraceCaches()
	require.Same(t, cache, f.handler.preConfirmedTraceCaches[20])
	cached := f.trace(t.Context(), 1)
	require.Nil(t, cached.err)
	require.Equal(t, first.trace, cached.trace)
	require.Equal(t, "0", cached.header.Get(ExecutionStepsHeader))
	require.Equal(t, uint64(1), f.opens.Load())
	require.Nil(t, f.trace(t.Context(), 2).err)
	require.Equal(t, uint64(2), executions.Load())
	f.apply(t, preConfirmedTestBlock("new block", 3), 21, 0)
	f.handler.refreshPreConfirmedTraceCaches()
	require.Same(t, cache, f.handler.preConfirmedTraceCaches[20])
	require.Nil(t, f.trace(t.Context(), 1).err)
	require.Equal(t, uint64(2), executions.Load())
	require.Nil(t, f.trace(t.Context(), 3).err)
	require.Equal(t, uint64(3), executions.Load())
	require.Equal(t, f.opens.Load(), f.closes.Load())
}

func TestPreConfirmedTraceInvalidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testing.T, *preConfirmedTestFixture)
	}{
		{"full replacement", func(t *testing.T, f *preConfirmedTestFixture) {
			f.apply(t, preConfirmedTestBlock("replacement", 1), 20, 0)
		}},
		{"base hash at same height", func(_ *testing.T, f *preConfirmedTestFixture) {
			f.baseHash.Store(felt.NewFromUint64[felt.Felt](100))
		}},
		{"revealed hash", func(_ *testing.T, f *preConfirmedTestFixture) {
			f.revealedHash.Store(felt.NewFromUint64[felt.Felt](81))
		}},
		{"revealed hash absent", func(_ *testing.T, f *preConfirmedTestFixture) {
			f.revealedHash.Store(nil)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newPreConfirmedTestFixture(t, &progressiveTestVM{trace: func(
				txs []core.Transaction, _ core.StateReader,
			) (vm.ExecutionResults, error) {
				return progressiveTestResults(txs), nil
			}})
			f.apply(t, preConfirmedTestBlock("round", 1), 20, 0)
			require.Nil(t, f.trace(t.Context(), 1).err)
			test.change(t, f)
			f.handler.refreshPreConfirmedTraceCaches()
			require.Empty(t, f.handler.preConfirmedTraceCaches)
			require.Nil(t, f.trace(t.Context(), 1).err)
			require.Equal(t, uint64(2), f.opens.Load())
			require.Equal(t, f.opens.Load(), f.closes.Load())
		})
	}
}

func TestPreConfirmedTraceAncestorAndCanonicalAdvance(t *testing.T) {
	f := newPreConfirmedTestFixture(t, &progressiveTestVM{trace: func(
		txs []core.Transaction, _ core.StateReader,
	) (vm.ExecutionResults, error) {
		return progressiveTestResults(txs), nil
	}})
	f.apply(t, preConfirmedTestBlock("first", 1), 20, 0)
	f.apply(t, preConfirmedTestBlock("second", 2), 21, 0)
	require.Nil(t, f.trace(t.Context(), 2).err)
	f.apply(t, preConfirmedTestBlock("replaced first", 3), 20, 0)
	f.handler.refreshPreConfirmedTraceCaches()
	require.Empty(t, f.handler.preConfirmedTraceCaches)
	require.NotNil(t, f.trace(t.Context(), 2).err, "removed descendants cannot use cached traces")
	f.apply(t, preConfirmedTestBlock("second", 2), 21, 0)
	require.Nil(t, f.trace(t.Context(), 2).err)
	require.Equal(t, uint64(2), f.opens.Load())
	require.True(t, f.storage.AdvanceTo(21))
	f.oldest.Store(21)
	f.baseHash.Store(felt.NewFromUint64[felt.Felt](101))
	f.handler.refreshPreConfirmedTraceCaches()
	require.Empty(t, f.handler.preConfirmedTraceCaches)
	require.Nil(t, f.trace(t.Context(), 2).err)
	require.Equal(t, uint64(3), f.opens.Load())
}

func TestPreConfirmedTraceFlightAcrossDelta(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var executions atomic.Uint64
	f := newPreConfirmedTestFixture(t, &progressiveTestVM{trace: func(
		txs []core.Transaction, _ core.StateReader,
	) (vm.ExecutionResults, error) {
		executions.Add(1)
		close(entered)
		<-release
		return progressiveTestResults(txs), nil
	}})
	f.apply(t, preConfirmedTestBlock("round", 1), 20, 0)
	owner := make(chan preConfirmedTestResult, 1)
	go func() { owner <- f.trace(t.Context(), 1) }()
	<-entered
	f.apply(t, preConfirmedTestDelta("round", 2), 20, 1)
	ctx := &traceWaitContext{Context: t.Context(), waiting: make(chan struct{})}
	waiter := make(chan preConfirmedTestResult, 1)
	go func() { waiter <- f.trace(ctx, 1) }()
	<-ctx.waiting
	require.Equal(t, uint64(1), f.opens.Load())
	close(release)
	first, second := <-owner, <-waiter
	require.Nil(t, first.err)
	require.Nil(t, second.err)
	require.Equal(t, first.trace, second.trace)
	require.Equal(t, "0", second.header.Get(ExecutionStepsHeader))
	require.Equal(t, uint64(1), executions.Load())
}

func TestPreConfirmedTraceOldFlightCannotOverwriteReplacement(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var executions atomic.Uint64
	f := newPreConfirmedTestFixture(t, &progressiveTestVM{trace: func(
		txs []core.Transaction, _ core.StateReader,
	) (vm.ExecutionResults, error) {
		call := executions.Add(1)
		if call == 1 {
			close(entered)
			<-release
		}
		result := progressiveTestResults(txs)
		result.GasConsumed[0].L1Gas = call
		return result, nil
	}})
	f.apply(t, preConfirmedTestBlock("round", 1), 20, 0)
	owner := make(chan preConfirmedTestResult, 1)
	go func() { owner <- f.trace(t.Context(), 1) }()
	<-entered
	oldCache := f.handler.preConfirmedTraceCaches[20]
	waiter := startTraceWaiter(t, oldCache, 0, t.Context())
	f.apply(t, preConfirmedTestBlock("replacement", 1), 20, 0)
	replacement := f.trace(t.Context(), 1)
	require.Nil(t, replacement.err)
	require.NotSame(t, oldCache, f.handler.preConfirmedTraceCaches[20])
	require.Equal(t, uint64(2), replacement.trace.ExecutionResources.L1Gas)
	close(release)
	require.Nil(t, (<-owner).err)
	require.Nil(t, (<-waiter).err)
	require.Len(t, f.handler.preConfirmedTraceCaches, 1)
	cached := f.trace(t.Context(), 1)
	require.Nil(t, cached.err)
	require.Equal(t, replacement.trace, cached.trace)
	require.Equal(t, uint64(2), executions.Load())
}

func TestPreConfirmedTraceMissingClassRetries(t *testing.T) {
	var executions atomic.Uint64
	f := newPreConfirmedTestFixture(t, &progressiveTestVM{trace: func(
		txs []core.Transaction, _ core.StateReader,
	) (vm.ExecutionResults, error) {
		executions.Add(1)
		return progressiveTestResults(txs), nil
	}})
	block := preConfirmedTestBlock("round", 1)
	block.Transactions[0].Type = starknet.TxnDeclare
	block.Transactions[0].ClassHash = &felt.One
	f.apply(t, block, 20, 0)
	f.state.EXPECT().Class(&felt.One).Return(nil, db.ErrKeyNotFound)
	require.NotNil(t, f.trace(t.Context(), 1).err)
	require.Zero(t, executions.Load())
	f.state.EXPECT().Class(&felt.One).Return(&core.DeclaredClassDefinition{
		Class: &core.SierraClass{},
	}, nil)
	require.Nil(t, f.trace(t.Context(), 1).err)
	require.Nil(t, f.trace(t.Context(), 1).err)
	require.Equal(t, uint64(1), executions.Load())
	require.Equal(t, uint64(2), f.opens.Load())
	require.Equal(t, f.opens.Load(), f.closes.Load())
}

func TestPreConfirmedTraceRetiresOnHeadNotification(t *testing.T) {
	f := newPreConfirmedTestFixture(t, &progressiveTestVM{trace: func(
		txs []core.Transaction, _ core.StateReader,
	) (vm.ExecutionResults, error) {
		return progressiveTestResults(txs), nil
	}})
	f.apply(t, preConfirmedTestBlock("round", 1), 20, 0)
	require.Nil(t, f.trace(t.Context(), 1).err)
	require.Len(t, f.handler.preConfirmedTraceCaches, 1)

	events := newFakeSyncer()
	syncReader := f.handler.syncReader.(*mocks.MockSyncReader)
	syncReader.EXPECT().SubscribeNewHeads().Return(events.SubscribeNewHeads())
	syncReader.EXPECT().SubscribeReorg().Return(events.SubscribeReorg())
	syncReader.EXPECT().SubscribePreConfirmed().Return(events.SubscribePreConfirmed())
	ready := make(chan struct{})
	f.handler.bcReader.(*mocks.MockReader).EXPECT().SubscribeL1Head().DoAndReturn(
		func() blockchain.L1HeadSubscription {
			close(ready)
			return blockchain.L1HeadSubscription{Subscription: feed.New[*core.L1Head]().Subscribe()}
		})
	forwarded := f.handler.newHeads.Subscribe()
	defer forwarded.Unsubscribe()
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() { finished <- f.handler.Run(ctx) }()
	<-ready
	require.True(t, f.storage.AdvanceTo(21))
	f.oldest.Store(21)
	events.newHeads.Send(&core.Block{Header: &core.Header{Number: 20}})
	<-forwarded.Recv() // Forwarding happens after reconciliation, without another trace request.
	cancel()
	require.NoError(t, <-finished)
	require.Empty(t, f.handler.preConfirmedTraceCaches)
	require.Equal(t, uint64(1), f.opens.Load())
}
