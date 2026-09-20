package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSharedTraceCacheInitialReads(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	runner := mocks.NewMockVM(ctrl)
	state := mocks.NewMockStateReader(ctrl)
	header := &core.Header{
		Hash:            felt.NewFromUint64[felt.Felt](100),
		ParentHash:      felt.NewFromUint64[felt.Felt](99),
		ProtocolVersion: "0.14.0",
	}
	transactions := []core.Transaction{
		&core.InvokeTransaction{
			Version:         new(core.TransactionVersion),
			TransactionHash: felt.NewFromUint64[felt.Felt](1),
		},
		&core.InvokeTransaction{
			Version:         new(core.TransactionVersion),
			TransactionHash: felt.NewFromUint64[felt.Felt](2),
		},
	}
	reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	reader.EXPECT().BlockHeaderByHash(header.Hash).Return(header, nil).AnyTimes()
	reader.EXPECT().BlockHeaderByNumber(header.Number).Return(header, nil).AnyTimes()
	for i := range transactions {
		reader.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(transactions[i].Hash()),
		).Return(header.Number, uint64(i), nil).AnyTimes()
	}
	reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(transactions, nil).Times(2)
	reader.EXPECT().StateAtBlockHash(header.ParentHash).
		Return(state, func() error { return nil }, nil).Times(2)
	reader.EXPECT().HeadState().Return(state, func() error { return nil }, nil).Times(2)
	// v9 executes the full block; v10 reuses it, then initial reads replay once.
	calls := 0
	runner.EXPECT().Trace(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(
		txs []core.Transaction,
		_ []core.ClassDefinition,
		_ []*felt.Felt,
		_ *vm.BlockInfo,
		_ core.StateReader,
		opts vm.TraceOptions,
	) (vm.ExecutionResults, error) {
		calls++
		require.Equal(t, transactions, txs)
		require.Equal(t, calls == 2, opts.ReturnInitialReads)
		result := vm.ExecutionResults{
			Traces:      make([]vm.TransactionTrace, len(txs)),
			GasConsumed: make([]core.GasConsumed, len(txs)),
			NumSteps:    11,
		}
		for i := range txs {
			result.Traces[i] = vm.TransactionTrace{
				Type:      vm.TxnInvoke,
				StateDiff: &vm.StateDiff{},
			}
			result.GasConsumed[i].L1Gas = txs[i].Hash().Uint64()
		}
		if opts.ReturnInitialReads {
			result.InitialReads = &vm.InitialReads{}
		}
		return result, nil
	}).Times(2)
	h := rpc.New(reader, nil, runner, "test", log.NewNopZapLogger(), &networks.Mainnet)
	methods := registeredTraceMethods(t, h)
	first, steps, err := methods.transaction9(
		t.Context(), (*felt.TransactionHash)(transactions[0].Hash()),
	)
	require.Nil(t, err)
	require.Equal(t, "11", steps.Get(rpcv9.ExecutionStepsHeader))
	require.Equal(t, uint64(1), first.ExecutionResources.L1Gas)
	second, steps, err := methods.transaction10(
		t.Context(), (*felt.TransactionHash)(transactions[1].Hash()),
	)
	require.Nil(t, err)
	require.Equal(t, "0", steps.Get(rpcv9.ExecutionStepsHeader))
	require.Equal(t, uint64(2), second.ExecutionResources.L1Gas)
	id10 := rpcv10.BlockIDFromHash(header.Hash)
	withReads, _, err := methods.block10(
		t.Context(),
		&id10,
		[]rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
	)
	require.Nil(t, err)
	require.NotNil(t, withReads.InitialReads)
	require.Len(t, withReads.Traces, 2)
	require.Equal(t, uint64(1), withReads.Traces[0].TraceRoot.ExecutionResources.L1Gas)
	require.Equal(t, uint64(2), withReads.Traces[1].TraceRoot.ExecutionResources.L1Gas)
	_, steps, err = methods.transaction9(t.Context(), (*felt.TransactionHash)(transactions[0].Hash()))
	require.Nil(t, err)
	require.Equal(t, "0", steps.Get(rpcv9.ExecutionStepsHeader))
	require.Equal(t, 2, calls)
}

func TestSharedFeederTraceCachePreservesVersionShapes(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	feeder := mocks.NewMockFeederReader(ctrl)
	header := &core.Header{
		Hash:             felt.NewFromUint64[felt.Felt](100),
		ProtocolVersion:  "0.12.0",
		TransactionCount: 2,
	}
	txs := []core.Transaction{
		&core.DeployTransaction{
			Version:         new(core.TransactionVersion),
			TransactionHash: felt.NewFromUint64[felt.Felt](1),
		},
		&core.L1HandlerTransaction{
			Version:         new(core.TransactionVersion),
			TransactionHash: felt.NewFromUint64[felt.Felt](2),
		},
	}
	blockTrace := starknet.BlockTrace{Traces: []starknet.TransactionTrace{
		{TransactionHash: *txs[0].Hash(), FunctionInvocation: &starknet.FunctionInvocation{}},
		{
			TransactionHash:    *txs[1].Hash(),
			FunctionInvocation: &starknet.FunctionInvocation{},
			RevertError:        "reverted",
		},
	}}
	reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	reader.EXPECT().BlockHeaderByHash(header.Hash).Return(header, nil).AnyTimes()
	reader.EXPECT().TransactionsAndReceiptsByBlockNumber(header.Number).Return(txs, nil, nil)
	feeder.EXPECT().BlockTrace(gomock.Any(), header.Hash.String()).Return(blockTrace, nil)
	h := rpc.New(reader, nil, nil, "test", log.NewNopZapLogger(), &networks.Mainnet).
		WithFeeder(feeder)
	methods := registeredTraceMethods(t, h)
	id9 := rpcv9.BlockIDFromHash(header.Hash)
	b, steps, err := methods.block9(t.Context(), &id9)
	require.Nil(t, err)
	require.Equal(t, "0", steps.Get(rpcv9.ExecutionStepsHeader))
	require.Equal(t, rpcv9.TxnDeploy, b[0].TraceRoot.Type)
	require.Equal(t, "reverted", b[1].TraceRoot.FunctionInvocation.RevertReason)
	id10 := rpcv10.BlockIDFromHash(header.Hash)
	for range 2 {
		c, steps, err := methods.block10(
			t.Context(),
			&id10,
			[]rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
		)
		require.Nil(t, err)
		require.Equal(t, "0", steps.Get(rpcv10.ExecutionStepsHeader))
		require.Equal(t, rpcv10.TxnDeploy, c.Traces[0].TraceRoot.Type)
		require.Equal(t, "reverted", c.Traces[1].TraceRoot.FunctionInvocation.RevertReason)
		reads, marshalErr := json.Marshal(c.InitialReads)
		require.NoError(t, marshalErr)
		require.JSONEq(
			t,
			`{"storage":null,"nonces":null,"class_hashes":null,"declared_contracts":null}`,
			string(reads),
		)
	}
}

func TestSharedTraceCacheCoordinatesConcurrentVersions(t *testing.T) {
	for _, useFeeder := range []bool{false, true} {
		t.Run(fmt.Sprintf("feeder=%t", useFeeder), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				reader := mocks.NewMockReader(ctrl)
				runner := mocks.NewMockVM(ctrl)
				state := mocks.NewMockStateReader(ctrl)
				feeder := mocks.NewMockFeederReader(ctrl)
				header := &core.Header{
					Hash:            felt.NewFromUint64[felt.Felt](100),
					ParentHash:      felt.NewFromUint64[felt.Felt](99),
					ProtocolVersion: "0.14.0",
				}
				txs := []core.Transaction{
					&core.InvokeTransaction{
						Version:         new(core.TransactionVersion),
						TransactionHash: felt.NewFromUint64[felt.Felt](1),
					},
				}
				reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
				reader.EXPECT().BlockHeaderByHash(header.Hash).Return(header, nil).Times(2)
				entered, release := make(chan struct{}), make(chan struct{})
				vmResult := vm.ExecutionResults{
					Traces: []vm.TransactionTrace{{
						Type: vm.TxnInvoke,
						ValidateInvocation: &vm.FunctionInvocation{
							Calls: []vm.FunctionInvocation{
								{ExecutionResources: &vm.ExecutionResources{L1Gas: 3}},
							},
						},
						StateDiff:          &vm.StateDiff{},
						ExecutionResources: &vm.ExecutionResources{L1Gas: 99},
					}},
					GasConsumed: []core.GasConsumed{{L1Gas: 7}},
					NumSteps:    11,
				}
				feederResult := starknet.BlockTrace{Traces: []starknet.TransactionTrace{{
					TransactionHash: *txs[0].Hash(),
					ValidateInvocation: &starknet.FunctionInvocation{
						InternalCalls: []starknet.FunctionInvocation{{ContractAddress: felt.One}},
					},
				}}}
				before, err := json.Marshal([]any{vmResult, feederResult})
				require.NoError(t, err)
				if useFeeder {
					header.ProtocolVersion = "0.12.0"
					header.TransactionCount = uint64(len(txs))
					reader.EXPECT().TransactionsAndReceiptsByBlockNumber(header.Number).
						Return(txs, nil, nil)
					feeder.EXPECT().BlockTrace(gomock.Any(), header.Hash.String()).
						DoAndReturn(func(context.Context, string) (starknet.BlockTrace, error) {
							close(entered)
							<-release
							return feederResult, nil
						})
				} else {
					reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil)
					reader.EXPECT().StateAtBlockHash(header.ParentHash).
						Return(state, func() error { return nil }, nil)
					reader.EXPECT().HeadState().
						Return(state, func() error { return nil }, nil)
					runner.EXPECT().Trace(
						txs, gomock.Any(), gomock.Any(), gomock.Any(), state, vm.TraceOptions{},
					).DoAndReturn(func(
						[]core.Transaction,
						[]core.ClassDefinition,
						[]*felt.Felt,
						*vm.BlockInfo,
						core.StateReader,
						vm.TraceOptions,
					) (vm.ExecutionResults, error) {
						close(entered)
						<-release
						return vmResult, nil
					})
				}
				h := rpc.New(reader, nil, runner, "test", log.NewNopZapLogger(), &networks.Mainnet).
					WithFeeder(feeder)
				methods := registeredTraceMethods(t, h)
				type response struct {
					steps      string
					err        *jsonrpc.Error
					marshalErr error
				}
				owner := make(chan response, 1)
				go func() {
					id := rpcv9.BlockIDFromHash(header.Hash)
					traces, steps, err := methods.block9(t.Context(), &id)
					_, marshalErr := json.Marshal(traces)
					owner <- response{steps.Get(rpcv9.ExecutionStepsHeader), err, marshalErr}
				}()
				<-entered
				done := make(chan response, 1)
				go func() {
					id := rpcv10.BlockIDFromHash(header.Hash)
					traces, steps, err := methods.block10(t.Context(), &id, nil)
					_, marshalErr := json.Marshal(traces)
					done <- response{steps.Get(rpcv10.ExecutionStepsHeader), err, marshalErr}
				}()
				synctest.Wait()
				require.Empty(t, done, "requests must wait for the producer")
				close(release)
				result := <-owner
				require.Nil(t, result.err)
				require.NoError(t, result.marshalErr)
				if useFeeder {
					require.Equal(t, "0", result.steps)
				} else {
					require.Equal(t, "11", result.steps)
				}
				result = <-done
				require.Nil(t, result.err)
				require.NoError(t, result.marshalErr)
				require.Equal(t, "0", result.steps)
				after, err := json.Marshal([]any{vmResult, feederResult})
				require.NoError(t, err)
				require.Equal(t, string(before), string(after))
			})
		})
	}
}

func TestSharedTraceCacheReleasesFailedProducer(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	header := &core.Header{
		Hash:            felt.NewFromUint64[felt.Felt](100),
		ParentHash:      felt.NewFromUint64[felt.Felt](99),
		ProtocolVersion: "0.14.0",
	}
	txs := []core.Transaction{
		&core.InvokeTransaction{
			Version:         new(core.TransactionVersion),
			TransactionHash: felt.NewFromUint64[felt.Felt](1),
		},
	}
	reader.EXPECT().Network().Return(&networks.Mainnet).Times(2)
	reader.EXPECT().BlockHeaderByHash(header.Hash).Return(header, nil).Times(2)
	reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil).Times(2)
	reader.EXPECT().StateAtBlockHash(header.ParentHash).
		Return(nil, nil, errors.New("state read failed")).Times(2)
	h := rpc.New(reader, nil, nil, "test", log.NewNopZapLogger(), &networks.Mainnet)
	methods := registeredTraceMethods(t, h)
	id9 := rpcv9.BlockIDFromHash(header.Hash)
	_, _, err := methods.block9(t.Context(), &id9)
	require.Equal(t, rpccore.ErrInternal.Code, err.Code)
	id10 := rpcv10.BlockIDFromHash(header.Hash)
	_, _, err = methods.block10(t.Context(), &id10, nil)
	require.Equal(t, rpccore.ErrInternal.Code, err.Code)
}

func TestSharedTraceCacheEachProducer(t *testing.T) {
	for _, useFeeder := range []bool{false, true} {
		for producer := range 2 {
			t.Run(fmt.Sprintf("feeder=%t/producer=%d", useFeeder, producer), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				reader := mocks.NewMockReader(ctrl)
				runner := mocks.NewMockVM(ctrl)
				state := mocks.NewMockStateReader(ctrl)
				feeder := mocks.NewMockFeederReader(ctrl)
				header := &core.Header{
					Hash:             felt.NewFromUint64[felt.Felt](100),
					ParentHash:       felt.NewFromUint64[felt.Felt](99),
					ProtocolVersion:  "0.14.0",
					TransactionCount: 1,
				}
				txs := []core.Transaction{
					&core.InvokeTransaction{
						TransactionHash: &felt.One,
						Version:         new(core.TransactionVersion),
					},
				}
				receipts := []*core.TransactionReceipt{{
					TransactionHash: &felt.One,
					ExecutionResources: &core.ExecutionResources{
						TotalGasConsumed: &core.GasConsumed{L1Gas: 7},
					},
				}}
				reader.EXPECT().BlockHeaderByHash(header.Hash).
					Return(header, nil).AnyTimes()
				reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
				if useFeeder {
					header.ProtocolVersion = "0.12.0"
					reader.EXPECT().TransactionsAndReceiptsByBlockNumber(header.Number).
						Return(txs, receipts, nil)
					feeder.EXPECT().BlockTrace(gomock.Any(), header.Hash.String()).
						Return(starknet.BlockTrace{
							Traces: []starknet.TransactionTrace{{TransactionHash: felt.One}},
						}, nil)
				} else {
					reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil)
					reader.EXPECT().StateAtBlockHash(header.ParentHash).
						Return(state, func() error { return nil }, nil)
					reader.EXPECT().HeadState().
						Return(state, func() error { return nil }, nil)
					runner.EXPECT().Trace(
						txs, gomock.Any(), gomock.Any(), gomock.Any(), state, vm.TraceOptions{},
					).Return(vm.ExecutionResults{
						Traces: []vm.TransactionTrace{
							{Type: vm.TxnInvoke, StateDiff: &vm.StateDiff{}},
						},
						GasConsumed: []core.GasConsumed{{L1Gas: 7}},
						NumSteps:    11,
					}, nil)
				}
				h := rpc.New(reader, nil, runner, "test", log.NewNopZapLogger(), &networks.Mainnet).
					WithFeeder(feeder)
				methods := registeredTraceMethods(t, h)
				for offset := range 2 {
					var steps http.Header
					var rpcErr *jsonrpc.Error
					var gas uint64
					switch (producer + offset) % 2 {
					case 0:
						id := rpcv9.BlockIDFromHash(header.Hash)
						var result []rpcv9.TracedBlockTransaction
						result, steps, rpcErr = methods.block9(t.Context(), &id)
						require.Nil(t, rpcErr)
						require.Len(t, result, 1)
						gas = result[0].TraceRoot.ExecutionResources.L1Gas
					case 1:
						id := rpcv10.BlockIDFromHash(header.Hash)
						var result rpcv10.TraceBlockTransactionsResponse
						result, steps, rpcErr = methods.block10(t.Context(), &id, nil)
						require.Nil(t, rpcErr)
						require.Len(t, result.Traces, 1)
						gas = result.Traces[0].TraceRoot.ExecutionResources.L1Gas
					}
					require.Equal(t, uint64(7), gas)
					expectedSteps := "0"
					if offset == 0 && !useFeeder {
						expectedSteps = "11"
					}
					require.Equal(t, expectedSteps, steps.Get(rpcv10.ExecutionStepsHeader))
				}
			})
		}
	}
}

type traceMethods struct {
	block9 func(context.Context, *rpcv9.BlockID) (
		[]rpcv9.TracedBlockTransaction,
		http.Header,
		*jsonrpc.Error,
	)
	block10 func(context.Context, *rpcv10.BlockID, []rpcv10.TraceFlag) (
		rpcv10.TraceBlockTransactionsResponse,
		http.Header,
		*jsonrpc.Error,
	)
	transaction9 func(context.Context, *felt.TransactionHash) (
		rpcv9.TransactionTrace,
		http.Header,
		*jsonrpc.Error,
	)
	transaction10 func(context.Context, *felt.TransactionHash) (
		rpcv10.TransactionTrace,
		http.Header,
		*jsonrpc.Error,
	)
}

func registeredTraceMethods(t *testing.T, h *rpc.Handler) traceMethods {
	t.Helper()
	v9, _ := h.MethodsV0_9()
	v10, _ := h.MethodsV0_10()
	var methods traceMethods
	bindTraceMethod(t, v9, "starknet_traceBlockTransactions", &methods.block9)
	bindTraceMethod(t, v10, "starknet_traceBlockTransactions", &methods.block10)
	bindTraceMethod(t, v9, "starknet_traceTransaction", &methods.transaction9)
	bindTraceMethod(t, v10, "starknet_traceTransaction", &methods.transaction10)
	return methods
}

func bindTraceMethod[T any](t *testing.T, methods []jsonrpc.Method, name string, target *T) {
	t.Helper()
	for _, method := range methods {
		if method.Name == name {
			handler, ok := method.Handler.(T)
			require.True(t, ok, "%s has unexpected handler type %T", name, method.Handler)
			*target = handler
			return
		}
	}
	t.Fatalf("method %s is not registered", name)
}
