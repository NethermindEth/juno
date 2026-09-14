package rpcv10

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/rpc/tracecache"
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

func progressiveHandler(
	t *testing.T, runner vm.VM,
) (*Handler, *core.Header, []core.Transaction, *mocks.MockStateReader) {
	t.Helper()
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
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
	}
	reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil).AnyTimes()
	reader.EXPECT().StateAtBlockHash(header.ParentHash).
		Return(parent, func() error { return nil }, nil).AnyTimes()
	reader.EXPECT().HeadState().Return(head, func() error { return nil }, nil).AnyTimes()
	return New(reader, nil, runner, log.NewNopZapLogger()), header, txs, head
}

func TestProgressiveFailureRetainsPrefixAndOffsetsError(t *testing.T) {
	var calls atomic.Uint64
	runner := &progressiveVM{run: func(
		txs []core.Transaction, _ core.StateReader, _ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		if calls.Add(1) == 2 {
			return vm.ExecutionResults{NumSteps: 9}, fmt.Errorf(
				"wrapped: %w", vm.TransactionExecutionError{Index: 0, Cause: json.RawMessage(`"failed"`)},
			)
		}
		require.Len(t, txs, 1)
		return progressiveResult(txs), nil
	}}
	h, header, txs, _ := progressiveHandler(t, runner)
	first := &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()}
	second := &tracecache.TransactionTarget{Index: 1, Hash: txs[1].Hash()}
	prefix, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, first)
	require.Nil(t, rpcErr)
	_, steps, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, second)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Data, "transaction #1")
	require.Equal(t, "9", steps.Get(ExecutionStepsHeader))
	hit, steps, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, first)
	require.Nil(t, rpcErr)
	require.Equal(t, prefix, hit)
	require.Equal(t, "0", steps.Get(ExecutionStepsHeader))
	next, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, second)
	require.Nil(t, rpcErr)
	require.Len(t, next.Traces, 2)
	require.Equal(t, uint64(3), calls.Load())
}

func TestProgressiveWaiterCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Uint64
	runner := &progressiveVM{run: func(
		txs []core.Transaction, _ core.StateReader, _ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls.Add(1)
		close(entered)
		<-release
		return progressiveResult(txs), nil
	}}
	h, header, txs, _ := progressiveHandler(t, runner)
	target := &tracecache.TransactionTarget{Index: 1, Hash: txs[1].Hash()}
	done := make(chan *jsonrpc.Error, 1)
	go func() {
		_, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
		done <- rpcErr
	}()
	<-entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, rpcErr := h.traceFinalisedBlock(ctx, header, false, target)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Data, context.Canceled.Error())
	close(release)
	require.Nil(t, <-done)
	result, steps, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
	require.Nil(t, rpcErr)
	require.Len(t, result.Traces, 2)
	require.Equal(t, "0", steps.Get(ExecutionStepsHeader))
	require.Equal(t, uint64(1), calls.Load())
}

func TestProgressivePanicAndMalformedResultsDoNotPublish(t *testing.T) {
	for _, failure := range []string{"panic", "traces", "gas", "state diff"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			runner := &progressiveVM{run: func(
				txs []core.Transaction, _ core.StateReader, _ vm.TraceOptions,
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
			h, header, txs, _ := progressiveHandler(t, runner)
			target := &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()}
			request := func() {
				_, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
				require.NotNil(t, rpcErr)
			}
			if failure == "panic" {
				require.PanicsWithValue(t, "execution panic", request)
			} else {
				request()
			}
			result, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
			require.Nil(t, rpcErr)
			require.Len(t, result.Traces, 1)
			require.Equal(t, 2, calls)
		})
	}
}

func TestProgressiveInitialReadsReplayPreservesPrefix(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Uint64
	runner := &progressiveVM{run: func(
		txs []core.Transaction, _ core.StateReader, opts vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		call := calls.Add(1)
		result := progressiveResult(txs)
		if call == 1 {
			require.Len(t, txs, 1)
			require.False(t, opts.ReturnInitialReads)
		} else {
			require.Len(t, txs, 3)
			require.True(t, opts.ReturnInitialReads)
			if call == 2 {
				close(entered)
				<-release
			} else {
				result.InitialReads = &vm.InitialReads{}
			}
		}
		return result, nil
	}}
	h, header, txs, _ := progressiveHandler(t, runner)
	target := &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()}
	prefix, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
	require.Nil(t, rpcErr)
	done := make(chan *jsonrpc.Error, 1)
	go func() {
		_, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, true, nil)
		done <- rpcErr
	}()
	<-entered
	hit, steps, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
	require.Nil(t, rpcErr)
	require.Equal(t, prefix, hit)
	require.Equal(t, "0", steps.Get(ExecutionStepsHeader))
	close(release)
	rpcErr = <-done
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Data, "VM omitted initial reads")
	hit, _, rpcErr = h.traceFinalisedBlock(t.Context(), header, false, target)
	require.Nil(t, rpcErr)
	require.Equal(t, prefix, hit)
	full, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, true, nil)
	require.Nil(t, rpcErr)
	require.Len(t, full.Traces, 3)
	require.NotNil(t, full.InitialReads)
	_, steps, rpcErr = h.traceFinalisedBlock(t.Context(), header, true, nil)
	require.Nil(t, rpcErr)
	require.Equal(t, "0", steps.Get(ExecutionStepsHeader))
	require.Equal(t, uint64(3), calls.Load())
}

func TestProgressiveCheckpointDeclarationsAndStorage(t *testing.T) {
	classHash := felt.FromUint64[felt.Felt](44)
	address := felt.FromUint64[felt.Felt](45)
	key := felt.FromUint64[felt.Felt](46)
	definition := &core.DeprecatedCairoClass{}
	calls := 0
	runner := &progressiveVM{run: func(
		txs []core.Transaction, state core.StateReader, _ vm.TraceOptions,
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
	h, header, txs, head := progressiveHandler(t, runner)
	head.EXPECT().Class(&classHash).Return(&core.DeclaredClassDefinition{Class: definition}, nil)
	_, _, rpcErr := h.traceFinalisedBlock(
		t.Context(), header, false, &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()},
	)
	require.Nil(t, rpcErr)
	_, _, rpcErr = h.traceFinalisedBlock(
		t.Context(), header, false, &tracecache.TransactionTarget{Index: 1, Hash: txs[1].Hash()},
	)
	require.Nil(t, rpcErr)
	require.Equal(t, 2, calls)
}

func TestProgressiveTargetIdentityBeforeExecutionAndOnHit(t *testing.T) {
	calls := 0
	runner := &progressiveVM{run: func(
		txs []core.Transaction, _ core.StateReader, _ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls++
		return progressiveResult(txs), nil
	}}
	h, header, txs, _ := progressiveHandler(t, runner)
	bad := &tracecache.TransactionTarget{Index: 0, Hash: felt.NewFromUint64[felt.Felt](999)}
	_, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, bad)
	require.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	require.Zero(t, calls)
	_, _, rpcErr = h.traceFinalisedBlock(
		t.Context(), header, false, &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()},
	)
	require.Nil(t, rpcErr)
	_, _, rpcErr = h.traceFinalisedBlock(t.Context(), header, false, bad)
	require.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	require.Equal(t, 1, calls)
	_, _, rpcErr = h.traceFinalisedBlock(
		t.Context(), header, false, &tracecache.TransactionTarget{Index: 3, Hash: txs[0].Hash()},
	)
	require.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	require.Equal(t, 1, calls)
}

func TestProgressiveCheckpointReadFailurePreservesPrefix(t *testing.T) {
	classHash := felt.FromUint64[felt.Felt](44)
	calls := 0
	runner := &progressiveVM{run: func(
		txs []core.Transaction, _ core.StateReader, _ vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls++
		result := progressiveResult(txs)
		result.Traces[0].StateDiff.DeprecatedDeclaredClasses = []*felt.Felt{&classHash}
		return result, nil
	}}
	h, header, txs, head := progressiveHandler(t, runner)
	head.EXPECT().Class(&classHash).Return(nil, errors.New("class read failed"))
	target := &tracecache.TransactionTarget{Index: 0, Hash: txs[0].Hash()}
	_, _, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
	require.Nil(t, rpcErr)
	_, _, rpcErr = h.traceFinalisedBlock(
		t.Context(), header, false, &tracecache.TransactionTarget{Index: 1, Hash: txs[1].Hash()},
	)
	require.NotNil(t, rpcErr)
	require.Equal(t, jsonrpc.InternalError, rpcErr.Code)
	_, steps, rpcErr := h.traceFinalisedBlock(t.Context(), header, false, target)
	require.Nil(t, rpcErr)
	require.Equal(t, "0", steps.Get(ExecutionStepsHeader))
	require.Equal(t, 1, calls)
}
