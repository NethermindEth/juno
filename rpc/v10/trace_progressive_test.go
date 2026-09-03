package rpcv10

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type progressiveTestVM struct {
	vm.VM
	trace func([]core.Transaction, core.StateReader) (vm.ExecutionResults, error)
}

func (v *progressiveTestVM) Trace(
	transactions []core.Transaction,
	_ []core.ClassDefinition,
	_ []*felt.Felt,
	_ *vm.BlockInfo,
	state core.StateReader,
	_ vm.TraceOptions,
) (vm.ExecutionResults, error) {
	return v.trace(transactions, state)
}

func progressiveTestTransactions(count int) []core.Transaction {
	transactions := make([]core.Transaction, count)
	for index := range transactions {
		transactions[index] = &core.InvokeTransaction{
			TransactionHash: felt.NewFromUint64[felt.Felt](uint64(index + 1)),
		}
	}
	return transactions
}

func progressiveTestResults(transactions []core.Transaction) vm.ExecutionResults {
	traces := make([]vm.TransactionTrace, len(transactions))
	gas := make([]core.GasConsumed, len(transactions))
	for index := range transactions {
		traces[index].StateDiff = &vm.StateDiff{}
		gas[index].L1Gas = transactions[index].Hash().Uint64()
	}
	return vm.ExecutionResults{
		Traces:       traces,
		GasConsumed:  gas,
		NumSteps:     uint64(len(transactions)),
		InitialReads: &vm.InitialReads{},
	}
}

func newProgressiveTestHandler(
	t *testing.T,
	virtualMachine vm.VM,
) (*Handler, *core.Header, []core.Transaction, *mocks.MockStateReader) {
	t.Helper()
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	state := mocks.NewMockStateReader(ctrl)
	header := &core.Header{
		Hash:             felt.NewFromUint64[felt.Felt](100),
		ParentHash:       felt.NewFromUint64[felt.Felt](99),
		SequencerAddress: felt.NewFromUint64[felt.Felt](98),
		L1GasPriceETH:    felt.NewFromUint64[felt.Felt](1),
		Number:           1,
		ProtocolVersion:  "99.12.3",
	}
	reader.EXPECT().StateAtBlockHash(header.ParentHash).
		Return(state, func() error { return nil }, nil).AnyTimes()
	reader.EXPECT().HeadState().Return(state, func() error { return nil }, nil).AnyTimes()
	handler := New(reader, nil, virtualMachine, log.NewNopZapLogger())
	return handler, header, progressiveTestTransactions(3), state
}

func TestMergeRPCStateDiff(t *testing.T) {
	address := felt.FromUint64[felt.Felt](1)
	key := felt.FromUint64[felt.Felt](2)
	value := felt.FromUint64[felt.Felt](3)
	nonce := felt.FromUint64[felt.Felt](4)
	classHash := felt.FromUint64[felt.Felt](5)
	compiledHash := felt.FromUint64[felt.Felt](6)
	replacement := felt.FromUint64[felt.Felt](7)
	migratedClass := felt.FromUint64[felt.SierraClassHash](8)
	migratedCompiled := felt.FromUint64[felt.CasmClassHash](9)

	converted := core.EmptyStateDiff()
	mergeRPCStateDiff(&converted, &StateDiff{
		StorageDiffs: []StorageDiff{{
			Address: address, StorageEntries: []Entry{{Key: key, Value: value}},
		}},
		Nonces:                    []Nonce{{ContractAddress: address, Nonce: nonce}},
		DeployedContracts:         []DeployedContract{{Address: address, ClassHash: classHash}},
		DeprecatedDeclaredClasses: []*felt.Felt{&classHash},
		DeclaredClasses: []DeclaredClass{{
			ClassHash: classHash, CompiledClassHash: compiledHash,
		}},
		ReplacedClasses: []ReplacedClass{{ContractAddress: address, ClassHash: replacement}},
		MigratedCompiledClasses: []MigratedCompiledClass{{
			ClassHash: migratedClass, CompiledClassHash: migratedCompiled,
		}},
	})

	require.Equal(t, value, *converted.StorageDiffs[address][key])
	require.Equal(t, nonce, *converted.Nonces[address])
	require.Equal(t, classHash, *converted.DeployedContracts[address])
	require.Equal(t, classHash, *converted.DeclaredV0Classes[0])
	require.Equal(t, compiledHash, *converted.DeclaredV1Classes[classHash])
	require.Equal(t, replacement, *converted.ReplacedClasses[address])
	require.Equal(t, migratedCompiled, converted.MigratedClasses[migratedClass])

	value.SetUint64(99)
	require.Equal(t, uint64(3), converted.StorageDiffs[address][key].Uint64())
}

func TestProgressiveTraceCacheWaiterCancellationDoesNotCancelExtension(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Uint64
	virtualMachine := &progressiveTestVM{trace: func(
		transactions []core.Transaction,
		_ core.StateReader,
	) (vm.ExecutionResults, error) {
		calls.Add(1)
		close(entered)
		<-release
		return progressiveTestResults(transactions), nil
	}}
	handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)

	ownerDone := make(chan *jsonrpc.Error, 1)
	go func() {
		_, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 1, false)
		ownerDone <- rpcErr
	}()
	<-entered
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, rpcErr := handler.traceProgressiveBlock(cancelled, header, transactions, 1, false)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Data, context.Canceled.Error())

	close(release)
	require.Nil(t, <-ownerDone)
	response, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 1, false)
	require.Nil(t, rpcErr)
	require.Len(t, response.Traces, 2)
	require.Equal(t, uint64(1), calls.Load())
}

func TestProgressiveTraceCacheFailureRetainsPublishedPrefix(t *testing.T) {
	var calls atomic.Uint64
	virtualMachine := &progressiveTestVM{trace: func(
		transactions []core.Transaction,
		_ core.StateReader,
	) (vm.ExecutionResults, error) {
		switch calls.Add(1) {
		case 2:
			transactionErr := vm.TransactionExecutionError{
				Index: 0,
				Cause: json.RawMessage(`"extension failed"`),
			}
			return vm.ExecutionResults{NumSteps: 9}, fmt.Errorf("VM decorator: %w", transactionErr)
		default:
			return progressiveTestResults(transactions), nil
		}
	}}
	handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)

	response, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 0, false)
	require.Nil(t, rpcErr)
	require.Len(t, response.Traces, 1)
	_, responseHeader, rpcErr := handler.traceProgressiveBlock(
		t.Context(), header, transactions, 1, false,
	)
	require.NotNil(t, rpcErr)
	require.Equal(t, "9", responseHeader.Get(ExecutionStepsHeader))
	require.Contains(t, rpcErr.Data, "transaction #1")

	response, responseHeader, rpcErr = handler.traceProgressiveBlock(
		t.Context(), header, transactions, 0, false,
	)
	require.Nil(t, rpcErr)
	require.Equal(t, "0", responseHeader.Get(ExecutionStepsHeader))
	require.Len(t, response.Traces, 1)
	require.Equal(t, uint64(2), calls.Load())

	response, _, rpcErr = handler.traceProgressiveBlock(t.Context(), header, transactions, 1, false)
	require.Nil(t, rpcErr)
	require.Len(t, response.Traces, 2)
	require.Equal(t, uint64(3), calls.Load())
}

func TestProgressiveTraceCacheDoesNotCacheFailedFirstExtension(t *testing.T) {
	virtualMachine := &progressiveTestVM{trace: func(
		[]core.Transaction,
		core.StateReader,
	) (vm.ExecutionResults, error) {
		return vm.ExecutionResults{}, vm.TransactionExecutionError{
			Index: 0,
			Cause: json.RawMessage(`"extension failed"`),
		}
	}}
	handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)

	_, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 0, false)
	require.NotNil(t, rpcErr)
	_, cached, inflight := blockTraceCacheState(handler.blockTraceCache, *header.Hash)
	require.False(t, cached)
	require.False(t, inflight)
}

func TestProgressiveTraceCacheRejectsMissingStateDiff(t *testing.T) {
	virtualMachine := &progressiveTestVM{trace: func(
		transactions []core.Transaction,
		_ core.StateReader,
	) (vm.ExecutionResults, error) {
		results := progressiveTestResults(transactions)
		results.Traces[0].StateDiff = nil
		return results, nil
	}}
	handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)

	_, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 0, false)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Data, "VM omitted state diff for transaction trace 0")

	_, cached, inflight := blockTraceCacheState(handler.blockTraceCache, *header.Hash)
	require.False(t, cached)
	require.False(t, inflight)
}

func TestProgressiveTraceCacheRejectsMismatchedVMResults(t *testing.T) {
	tests := []struct {
		name       string
		traceCount int
		gasCount   int
		want       string
	}{
		{"too few traces", 1, 2, "unexpected number of transaction traces"},
		{"too few gas results", 2, 1, "unexpected number of gas results"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			virtualMachine := &progressiveTestVM{trace: func(
				[]core.Transaction, core.StateReader,
			) (vm.ExecutionResults, error) {
				return vm.ExecutionResults{
					Traces:      make([]vm.TransactionTrace, test.traceCount),
					GasConsumed: make([]core.GasConsumed, test.gasCount),
				}, nil
			}}
			handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)
			_, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 1, false)
			require.NotNil(t, rpcErr)
			require.Contains(t, rpcErr.Data, test.want)
		})
	}
}

func TestProgressiveTraceCachePanicClearsInflight(t *testing.T) {
	var calls atomic.Uint64
	virtualMachine := &progressiveTestVM{trace: func(
		transactions []core.Transaction,
		_ core.StateReader,
	) (vm.ExecutionResults, error) {
		if calls.Add(1) == 1 {
			panic("trace panic")
		}
		return progressiveTestResults(transactions), nil
	}}
	handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)

	func() {
		defer func() {
			require.Equal(t, "trace panic", recover())
		}()
		_, _, _ = handler.traceProgressiveBlock(t.Context(), header, transactions, 0, false)
	}()

	response, _, rpcErr := handler.traceProgressiveBlock(
		t.Context(), header, transactions, 0, false,
	)
	require.Nil(t, rpcErr)
	require.Len(t, response.Traces, 1)
	require.Equal(t, uint64(2), calls.Load())
}

func TestTraceFinalisedEmptyBlockReturnsWithoutCaching(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	virtualMachine := mocks.NewMockVM(ctrl)
	header := &core.Header{
		Hash:            felt.NewFromUint64[felt.Felt](100),
		ProtocolVersion: "99.12.3",
	}

	reader.EXPECT().Network().Return(&networks.Mainnet).Times(2)
	reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(nil, nil).Times(2)
	handler := New(reader, nil, virtualMachine, log.NewNopZapLogger())

	response, responseHeader, rpcErr := handler.traceFinalisedBlock(
		t.Context(), header, true,
	)
	require.Nil(t, rpcErr)
	require.Empty(t, response.Traces)
	require.NotNil(t, response.InitialReads)
	require.Empty(t, response.InitialReads.Storage)
	require.Empty(t, response.InitialReads.Nonces)
	require.Empty(t, response.InitialReads.ClassHashes)
	require.Empty(t, response.InitialReads.DeclaredContracts)
	require.Equal(t, "0", responseHeader.Get(ExecutionStepsHeader))

	response, responseHeader, rpcErr = handler.traceFinalisedBlock(t.Context(), header, false)
	require.Nil(t, rpcErr)
	require.Empty(t, response.Traces)
	require.Nil(t, response.InitialReads)
	require.Equal(t, "0", responseHeader.Get(ExecutionStepsHeader))
}

func TestProgressiveTraceCacheAllowsRecordEvictionDuringFlight(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Uint64
	virtualMachine := &progressiveTestVM{trace: func(
		transactions []core.Transaction,
		_ core.StateReader,
	) (vm.ExecutionResults, error) {
		if calls.Add(1) == 2 {
			close(entered)
			<-release
		}
		return progressiveTestResults(transactions), nil
	}}
	handler, header, transactions, _ := newProgressiveTestHandler(t, virtualMachine)
	prefix, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 0, false)
	require.Nil(t, rpcErr)
	require.Len(t, prefix.Traces, 1)

	ownerDone := make(chan *jsonrpc.Error, 1)
	go func() {
		_, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 1, false)
		ownerDone <- rpcErr
	}()
	<-entered
	for index := range rpccore.TraceCacheSize {
		handler.blockTraceCache.storeComplete(
			felt.FromUint64[felt.Felt](uint64(1_000+index)),
			TraceBlockTransactionsResponse{InitialReads: emptyInitialReads()},
		)
	}
	_, found, inflight := blockTraceCacheState(handler.blockTraceCache, *header.Hash)
	require.False(t, found, "the base record may be evicted while its owner retains it")
	require.True(t, inflight, "active work must remain discoverable through its flight")

	waiting := handler.blockTraceCache.lookupOrStart(*header.Hash, 1)
	require.Equal(t, traceCacheWait, waiting.kind)
	close(release)

	require.Nil(t, <-ownerDone)
	select {
	case <-waiting.done:
	default:
		t.Fatal("commit must wake waiters after republishing the evicted record")
	}
	require.Equal(t, uint64(2), calls.Load())
	record, found, inflight := blockTraceCacheState(handler.blockTraceCache, *header.Hash)
	require.True(t, found)
	require.False(t, inflight)
	require.Len(t, record.traces, 2, "success should republish the extended record")
}

func TestProgressiveTraceCacheMakesPrefixDeclarationsAvailableToSuffix(t *testing.T) {
	classHash := felt.FromUint64[felt.Felt](44)
	classDefinition := &core.DeprecatedCairoClass{}
	var calls atomic.Uint64
	virtualMachine := &progressiveTestVM{trace: func(
		transactions []core.Transaction,
		state core.StateReader,
	) (vm.ExecutionResults, error) {
		results := progressiveTestResults(transactions)
		switch calls.Add(1) {
		case 1:
			results.Traces[0].StateDiff.DeprecatedDeclaredClasses = []*felt.Felt{&classHash}
		case 2:
			declared, err := state.Class(&classHash)
			require.NoError(t, err)
			require.Same(t, classDefinition, declared.Class)
		}
		return results, nil
	}}
	handler, header, transactions, headState := newProgressiveTestHandler(t, virtualMachine)
	headState.EXPECT().Class(&classHash).Return(&core.DeclaredClassDefinition{
		Class: classDefinition,
	}, nil)

	_, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 0, false)
	require.Nil(t, rpcErr)
	response, _, rpcErr := handler.traceProgressiveBlock(t.Context(), header, transactions, 1, false)
	require.Nil(t, rpcErr)
	require.Len(t, response.Traces, 2)
	require.Equal(t, uint64(2), calls.Load())
}

func TestTransactionTraceIfHashMatchesValidatesCachedEntry(t *testing.T) {
	hash := felt.FromUint64[felt.TransactionHash](1)
	otherHash := felt.FromUint64[felt.Felt](2)
	trace := &TransactionTrace{}
	tests := map[string]TracedBlockTransaction{
		"nil hash":      {TraceRoot: trace},
		"nil trace":     {TransactionHash: (*felt.Felt)(&hash)},
		"hash mismatch": {TransactionHash: &otherHash, TraceRoot: trace},
	}
	for name, cached := range tests {
		t.Run(name, func(t *testing.T) {
			_, valid := transactionTraceIfHashMatches(cached, &hash)
			require.False(t, valid)
		})
	}

	result, valid := transactionTraceIfHashMatches(TracedBlockTransaction{
		TransactionHash: (*felt.Felt)(&hash), TraceRoot: trace,
	}, &hash)
	require.True(t, valid)
	require.Equal(t, *trace, result)
}
