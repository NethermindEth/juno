package rpc

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/tracecache"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTraceTransactionSkipsUnrequestedAdaptation(t *testing.T) {
	for _, version := range []int{8, 9, 10} {
		for _, feeder := range []bool{false, true} {
			for _, cached := range []bool{false, true} {
				t.Run(fmt.Sprintf("v%d/feeder=%t/cached=%t", version, feeder, cached), func(t *testing.T) {
					// The requested transaction is identical. Only the other transaction's
					// invocation tree grows, so selecting first must keep allocations flat.
					small := testing.AllocsPerRun(10, traceAdaptationRequest(t, version, feeder, cached, 0))
					large := testing.AllocsPerRun(10, traceAdaptationRequest(t, version, feeder, cached, 128))
					t.Logf("allocations/request: small=%g large=%g", small, large)
					require.LessOrEqual(t, large, small+1, "unrequested invocation trees must not be adapted")
				})
			}
		}
	}
}

func traceAdaptationRequest(t *testing.T, version int, feeder, cached bool, calls int) func() {
	t.Helper()
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	runner := mocks.NewMockVM(ctrl)
	state := mocks.NewMockStateReader(ctrl)
	gateway := mocks.NewMockFeederReader(ctrl)
	header := &core.Header{
		Hash: felt.NewFromUint64[felt.Felt](100), ParentHash: felt.NewFromUint64[felt.Felt](99),
		ProtocolVersion: "0.14.0", TransactionCount: 2,
	}
	txs := []core.Transaction{
		&core.InvokeTransaction{TransactionHash: &felt.One, Version: new(core.TransactionVersion)},
		&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](2), Version: new(core.TransactionVersion)},
	}
	hash := txs[1].Hash()
	receipts := []*core.TransactionReceipt{{
		TransactionHash:    hash,
		ExecutionResources: &core.ExecutionResources{TotalGasConsumed: &core.GasConsumed{L1Gas: 7}},
	}}
	reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	reader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil).AnyTimes()
	reader.EXPECT().BlockByHash(header.Hash).Return(&core.Block{Header: header, Transactions: txs, Receipts: receipts}, nil).AnyTimes()
	reader.EXPECT().BlockNumberAndIndexByTxHash((*felt.TransactionHash)(hash)).Return(header.Number, uint64(1), nil).AnyTimes()
	reader.EXPECT().BlockHeaderByNumber(header.Number).Return(header, nil).AnyTimes()
	var record *tracecache.BlockTrace
	var err error
	if feeder {
		header.ProtocolVersion = "0.12.0"
		invocation := &starknet.FunctionInvocation{InternalCalls: make([]starknet.FunctionInvocation, calls)}
		result := starknet.BlockTrace{Traces: []starknet.TransactionTrace{
			{TransactionHash: *txs[0].Hash(), ValidateInvocation: invocation},
			{TransactionHash: *hash},
		}}
		record, err = tracecache.FromFeeder([]vm.TransactionType{vm.TxnInvoke, vm.TxnInvoke}, receipts, &result)
		if !cached {
			reader.EXPECT().BlockHeaderByHash(header.Hash).Return(header, nil).AnyTimes()
			reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil).AnyTimes()
			reader.EXPECT().TransactionsAndReceiptsByBlockNumber(header.Number).Return(txs, receipts, nil).AnyTimes()
			reader.EXPECT().L1Head().Return(core.L1Head{}, nil).AnyTimes()
			gateway.EXPECT().BlockTrace(gomock.Any(), header.Hash.String()).Return(result, nil).AnyTimes()
		}
	} else {
		invocation := &vm.FunctionInvocation{Calls: make([]vm.FunctionInvocation, calls)}
		for i := range invocation.Calls {
			invocation.Calls[i].ExecutionResources = &vm.ExecutionResources{L1Gas: 1}
		}
		result := vm.ExecutionResults{
			Traces: []vm.TransactionTrace{
				{Type: vm.TxnInvoke, ValidateInvocation: invocation, StateDiff: &vm.StateDiff{}},
				{Type: vm.TxnInvoke, StateDiff: &vm.StateDiff{}},
			},
			GasConsumed: []core.GasConsumed{{}, {L1Gas: 7}},
		}
		record, err = tracecache.FromVM(txs, &result, false)
		if !cached {
			reader.EXPECT().TransactionsByBlockNumber(header.Number).Return(txs, nil).AnyTimes()
			reader.EXPECT().StateAtBlockHash(header.ParentHash).Return(state, func() error { return nil }, nil).AnyTimes()
			reader.EXPECT().HeadState().Return(state, func() error { return nil }, nil).AnyTimes()
			runner.EXPECT().Trace(txs, gomock.Any(), gomock.Any(), gomock.Any(), state, vm.TraceOptions{}).Return(result, nil).AnyTimes()
		}
	}
	require.NoError(t, err)
	h := New(reader, nil, runner, "test", log.NewNopZapLogger(), &networks.Mainnet).WithFeeder(gateway)
	installCache := func() *tracecache.Cache[felt.Felt, *tracecache.BlockTrace] {
		cache := tracecache.New[felt.Felt, *tracecache.BlockTrace](1)
		h.rpcv8Handler.WithTraceCache(cache)
		h.rpcv9Handler.WithTraceCache(cache)
		h.rpcv10Handler.WithTraceCache(cache)
		return cache
	}
	if cached {
		cache := installCache()
		_, lease, cacheErr := cache.Acquire(t.Context(), *header.Hash, nil)
		require.NoError(t, cacheErr)
		lease.Publish(record)
	}
	return func() {
		if !cached {
			installCache()
		}
		switch version {
		case 8:
			trace, _, rpcErr := h.rpcv8Handler.TraceTransaction(t.Context(), *hash)
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(7), trace.ExecutionResources.L1Gas)
		case 9:
			trace, _, rpcErr := h.rpcv9Handler.TraceTransaction(t.Context(), (*felt.TransactionHash)(hash))
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(7), trace.ExecutionResources.L1Gas)
		case 10:
			trace, _, rpcErr := h.rpcv10Handler.TraceTransaction(t.Context(), (*felt.TransactionHash)(hash))
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(7), trace.ExecutionResources.L1Gas)
		}
	}
}
