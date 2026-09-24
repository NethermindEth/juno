package rpcv10_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type progressiveVM struct {
	vm.VM
	run func([]core.Transaction, core.StateReader, vm.TraceOptions) (vm.ExecutionResults, error)
}

func (v *progressiveVM) Trace(
	txs []core.Transaction,
	_ []core.ClassDefinition,
	_ []*felt.Felt,
	_ *vm.BlockInfo,
	state core.StateReader,
	opts vm.TraceOptions,
) (vm.ExecutionResults, error) {
	return v.run(txs, state, opts)
}

func progressiveResult(txs []core.Transaction) vm.ExecutionResults {
	result := vm.ExecutionResults{
		Traces:      make([]vm.TransactionTrace, len(txs)),
		GasConsumed: make([]core.GasConsumed, len(txs)),
		NumSteps:    uint64(len(txs)),
	}
	for i, tx := range txs {
		result.Traces[i] = vm.TransactionTrace{Type: vm.TxnInvoke, StateDiff: &vm.StateDiff{}}
		result.GasConsumed[i].L1Gas = tx.Hash().Uint64()
	}
	return result
}

type progressiveFixture struct {
	handler *rpcv10.Handler
	blockID rpcv10.BlockID
	txs     []core.Transaction
	head    *mocks.MockStateReader
	reader  *mocks.MockReader
}

func progressiveHandler(t *testing.T, runner vm.VM) progressiveFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	syncReader := mocks.NewMockSyncReader(ctrl)
	parent := mocks.NewMockStateReader(ctrl)
	head := mocks.NewMockStateReader(ctrl)
	header := &core.Header{
		Hash:            felt.NewFromUint64[felt.Felt](100),
		ParentHash:      felt.NewFromUint64[felt.Felt](99),
		ProtocolVersion: "0.14.0",
	}
	txs := make([]core.Transaction, 3)
	for i := range txs {
		txs[i] = &core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](uint64(i + 1))}
		reader.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(txs[i].Hash()),
		).Return(header.Number, uint64(i), nil).AnyTimes()
	}
	reader.EXPECT().BlockHeaderByNumber(header.Number).Return(header, nil).AnyTimes()
	reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil).AnyTimes()
	reader.EXPECT().StateAtBlockHash(header.ParentHash).
		Return(parent, func() error { return nil }, nil).AnyTimes()
	reader.EXPECT().HeadState().Return(head, func() error { return nil }, nil).AnyTimes()
	syncReader.EXPECT().PreConfirmedChain().
		Return(preconfirmed.ChainReader{}, db.ErrKeyNotFound).AnyTimes()
	return progressiveFixture{
		handler: rpcv10.New(reader, syncReader, runner, log.NewNopZapLogger()),
		blockID: rpcv10.BlockIDFromNumber(header.Number),
		txs:     txs,
		head:    head,
		reader:  reader,
	}
}

func TestProgressiveFailureRetainsPrefixAndOffsetsError(t *testing.T) {
	var calls atomic.Uint64
	runner := &progressiveVM{run: func(
		txs []core.Transaction,
		_ core.StateReader,
		_ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		if calls.Add(1) == 2 {
			return vm.ExecutionResults{NumSteps: 9}, fmt.Errorf(
				"wrapped: %w", vm.TransactionExecutionError{Index: 0, Cause: json.RawMessage(`"failed"`)},
			)
		}
		require.Len(t, txs, 1)
		return progressiveResult(txs), nil
	}}
	f := progressiveHandler(t, runner)
	first := (*felt.TransactionHash)(f.txs[0].Hash())
	second := (*felt.TransactionHash)(f.txs[1].Hash())
	prefix, _, rpcErr := f.handler.TraceTransaction(t.Context(), first)
	require.Nil(t, rpcErr)
	_, steps, rpcErr := f.handler.TraceBlockTransactions(t.Context(), &f.blockID, nil)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Data, "transaction #1")
	require.Equal(t, "9", steps.Get(rpcv10.ExecutionStepsHeader))
	hit, steps, rpcErr := f.handler.TraceTransaction(t.Context(), first)
	require.Nil(t, rpcErr)
	require.Equal(t, prefix, hit)
	require.Equal(t, "0", steps.Get(rpcv10.ExecutionStepsHeader))
	next, _, rpcErr := f.handler.TraceTransaction(t.Context(), second)
	require.Nil(t, rpcErr)
	require.Equal(t, uint64(2), next.ExecutionResources.L1Gas)
	require.Equal(t, uint64(3), calls.Load())
}

func TestProgressiveWaiterCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		releaseProducer := sync.OnceFunc(func() { close(release) })
		defer releaseProducer()
		var calls atomic.Uint64
		runner := &progressiveVM{run: func(
			txs []core.Transaction,
			_ core.StateReader,
			_ vm.TraceOptions,
		) (vm.ExecutionResults, error) {
			if calls.Add(1) == 1 {
				close(entered)
			}
			<-release
			return progressiveResult(txs), nil
		}}
		f := progressiveHandler(t, runner)
		target := (*felt.TransactionHash)(f.txs[1].Hash())
		owner := make(chan *jsonrpc.Error, 1)
		go func() {
			_, _, rpcErr := f.handler.TraceTransaction(t.Context(), target)
			owner <- rpcErr
		}()
		<-entered
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		waiter := make(chan *jsonrpc.Error, 1)
		go func() {
			_, _, rpcErr := f.handler.TraceTransaction(ctx, target)
			waiter <- rpcErr
		}()
		synctest.Wait()
		require.Empty(t, waiter, "request must wait for the producer")
		require.Equal(t, uint64(1), calls.Load())
		cancel()
		synctest.Wait()
		require.Len(t, waiter, 1, "cancellation must wake the waiter")
		rpcErr := <-waiter
		require.NotNil(t, rpcErr)
		require.Contains(t, rpcErr.Data, context.Canceled.Error())
		require.Empty(t, owner, "cancellation must leave the producer running")
		releaseProducer()
		require.Nil(t, <-owner)
		result, steps, rpcErr := f.handler.TraceTransaction(t.Context(), target)
		require.Nil(t, rpcErr)
		require.Equal(t, uint64(2), result.ExecutionResources.L1Gas)
		require.Equal(t, "0", steps.Get(rpcv10.ExecutionStepsHeader))
		require.Equal(t, uint64(1), calls.Load())
	})
}

func TestProgressivePanicAndMalformedResultsDoNotPublish(t *testing.T) {
	for _, failure := range []string{"panic", "traces", "gas", "state diff"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			runner := &progressiveVM{run: func(
				txs []core.Transaction,
				_ core.StateReader,
				_ vm.TraceOptions,
			) (vm.ExecutionResults, error) {
				calls++
				result := progressiveResult(txs)
				if calls == 1 {
					switch failure {
					case "panic":
						panic("execution panic")
					case "traces":
						result.Traces = nil
					case "gas":
						result.GasConsumed = nil
					case "state diff":
						result.Traces[0].StateDiff = nil
					}
				}
				return result, nil
			}}
			f := progressiveHandler(t, runner)
			target := (*felt.TransactionHash)(f.txs[0].Hash())
			request := func() {
				_, _, rpcErr := f.handler.TraceTransaction(t.Context(), target)
				require.NotNil(t, rpcErr)
			}
			if failure == "panic" {
				require.PanicsWithValue(t, "execution panic", request)
			} else {
				request()
			}
			result, _, rpcErr := f.handler.TraceTransaction(t.Context(), target)
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(1), result.ExecutionResources.L1Gas)
			require.Equal(t, 2, calls)
		})
	}
}

func TestProgressiveInitialReadsReplayPreservesPrefix(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		releaseProducer := sync.OnceFunc(func() { close(release) })
		defer releaseProducer()
		var calls atomic.Uint64
		type execution struct {
			transactions []core.Transaction
			initialReads bool
		}
		observed := make(chan execution, 3)
		runner := &progressiveVM{run: func(
			txs []core.Transaction,
			_ core.StateReader,
			opts vm.TraceOptions,
		) (vm.ExecutionResults, error) {
			call := calls.Add(1)
			observed <- execution{transactions: txs, initialReads: opts.ReturnInitialReads}
			result := progressiveResult(txs)
			if call == 2 {
				close(entered)
				<-release
			} else if call > 2 {
				result.InitialReads = &vm.InitialReads{}
			}
			return result, nil
		}}
		f := progressiveHandler(t, runner)
		target := (*felt.TransactionHash)(f.txs[0].Hash())
		prefix, _, rpcErr := f.handler.TraceTransaction(t.Context(), target)
		require.Nil(t, rpcErr)
		require.Equal(t, execution{transactions: f.txs[:1]}, <-observed)
		flags := []rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag}
		done := make(chan *jsonrpc.Error, 1)
		go func() {
			_, _, rpcErr := f.handler.TraceBlockTransactions(t.Context(), &f.blockID, flags)
			done <- rpcErr
		}()
		<-entered
		require.Equal(t, execution{transactions: f.txs, initialReads: true}, <-observed)
		hit, steps, rpcErr := f.handler.TraceTransaction(t.Context(), target)
		require.Nil(t, rpcErr)
		require.Equal(t, prefix, hit)
		require.Equal(t, "0", steps.Get(rpcv10.ExecutionStepsHeader))
		releaseProducer()
		rpcErr = <-done
		require.NotNil(t, rpcErr)
		require.Contains(t, rpcErr.Data, "VM omitted initial reads")
		hit, _, rpcErr = f.handler.TraceTransaction(t.Context(), target)
		require.Nil(t, rpcErr)
		require.Equal(t, prefix, hit)
		full, _, rpcErr := f.handler.TraceBlockTransactions(t.Context(), &f.blockID, flags)
		require.Nil(t, rpcErr)
		require.Equal(t, execution{transactions: f.txs, initialReads: true}, <-observed)
		require.Len(t, full.Traces, len(f.txs))
		require.NotNil(t, full.InitialReads)
		for i := range full.Traces {
			require.Equal(t, f.txs[i].Hash(), full.Traces[i].TransactionHash)
		}
		_, steps, rpcErr = f.handler.TraceBlockTransactions(t.Context(), &f.blockID, flags)
		require.Nil(t, rpcErr)
		require.Equal(t, "0", steps.Get(rpcv10.ExecutionStepsHeader))
		require.Equal(t, uint64(3), calls.Load())
	})
}

func TestProgressiveCheckpointDeclarationsAndStorage(t *testing.T) {
	classHash := felt.FromUint64[felt.Felt](44)
	address := felt.FromUint64[felt.Felt](45)
	key := felt.FromUint64[felt.Felt](46)
	definition := &core.DeprecatedCairoClass{}
	calls := 0
	runner := &progressiveVM{run: func(
		txs []core.Transaction,
		state core.StateReader,
		_ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls++
		result := progressiveResult(txs)
		if calls == 1 {
			result.Traces[0].StateDiff.DeprecatedDeclaredClasses = []*felt.Felt{&classHash}
			result.Traces[0].StateDiff.StorageDiffs = []vm.StorageDiff{{
				Address:        address,
				StorageEntries: []vm.Entry{{Key: key, Value: felt.One}},
			}}
		} else {
			declared, err := state.Class(&classHash)
			require.NoError(t, err)
			require.Same(t, definition, declared.Class)
			value, err := state.ContractStorage(&address, &key)
			require.NoError(t, err)
			require.Equal(t, felt.One, value)
		}
		return result, nil
	}}
	f := progressiveHandler(t, runner)
	f.head.EXPECT().Class(&classHash).Return(&core.DeclaredClassDefinition{Class: definition}, nil)
	_, _, rpcErr := f.handler.TraceTransaction(
		t.Context(), (*felt.TransactionHash)(f.txs[0].Hash()),
	)
	require.Nil(t, rpcErr)
	_, _, rpcErr = f.handler.TraceTransaction(
		t.Context(), (*felt.TransactionHash)(f.txs[1].Hash()),
	)
	require.Nil(t, rpcErr)
	require.Equal(t, 2, calls)
}

func TestProgressiveTargetIdentityBeforeExecutionAndOnHit(t *testing.T) {
	calls := 0
	runner := &progressiveVM{run: func(
		txs []core.Transaction,
		_ core.StateReader,
		_ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls++
		return progressiveResult(txs), nil
	}}
	f := progressiveHandler(t, runner)
	bad := felt.NewFromUint64[felt.TransactionHash](999)
	f.reader.EXPECT().BlockNumberAndIndexByTxHash(bad).
		Return(f.blockID.Number(), uint64(0), nil).Times(2)
	_, _, rpcErr := f.handler.TraceTransaction(t.Context(), bad)
	require.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	require.Zero(t, calls)
	_, _, rpcErr = f.handler.TraceTransaction(
		t.Context(), (*felt.TransactionHash)(f.txs[0].Hash()),
	)
	require.Nil(t, rpcErr)
	_, _, rpcErr = f.handler.TraceTransaction(t.Context(), bad)
	require.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	require.Equal(t, 1, calls)
	f.reader.EXPECT().BlockNumberAndIndexByTxHash(bad).
		Return(f.blockID.Number(), uint64(len(f.txs)), nil)
	_, _, rpcErr = f.handler.TraceTransaction(t.Context(), bad)
	require.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	require.Equal(t, 1, calls)
}

func TestProgressiveCheckpointReadFailurePreservesPrefix(t *testing.T) {
	classHash := felt.FromUint64[felt.Felt](44)
	calls := 0
	runner := &progressiveVM{run: func(
		txs []core.Transaction,
		_ core.StateReader,
		_ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls++
		result := progressiveResult(txs)
		result.Traces[0].StateDiff.DeprecatedDeclaredClasses = []*felt.Felt{&classHash}
		return result, nil
	}}
	f := progressiveHandler(t, runner)
	f.head.EXPECT().Class(&classHash).Return(nil, errors.New("class read failed"))
	target := (*felt.TransactionHash)(f.txs[0].Hash())
	_, _, rpcErr := f.handler.TraceTransaction(t.Context(), target)
	require.Nil(t, rpcErr)
	_, _, rpcErr = f.handler.TraceTransaction(
		t.Context(), (*felt.TransactionHash)(f.txs[1].Hash()),
	)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonrpc.InternalError, rpcErr.Code)
	_, steps, rpcErr := f.handler.TraceTransaction(t.Context(), target)
	require.Nil(t, rpcErr)
	require.Equal(t, "0", steps.Get(rpcv10.ExecutionStepsHeader))
	require.Equal(t, 1, calls)
}
