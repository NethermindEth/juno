package rpc_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/tracecache"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTraceTransactionSkipsUnrequestedAdaptation(t *testing.T) {
	for _, version := range []int{8, 9, 10} {
		for _, feeder := range []bool{false, true} {
			t.Run(fmt.Sprintf("v%d/feeder=%t", version, feeder), func(t *testing.T) {
				// Growing an unrelated invocation tree must not add adaptation allocations.
				small := testing.AllocsPerRun(10, traceAdaptationRequest(t, version, feeder, 0))
				large := testing.AllocsPerRun(10, traceAdaptationRequest(t, version, feeder, 128))
				require.LessOrEqual(t, large, small+1)
			})
		}
	}
}

func traceAdaptationRequest(t *testing.T, version int, feeder bool, calls int) func() {
	t.Helper()
	ctrl := gomock.NewController(t)
	reader := mocks.NewMockReader(ctrl)
	header := &core.Header{
		Hash:             felt.NewFromUint64[felt.Felt](100),
		ParentHash:       felt.NewFromUint64[felt.Felt](99),
		ProtocolVersion:  "0.14.0",
		TransactionCount: 2,
	}
	txs := []core.Transaction{
		&core.InvokeTransaction{
			TransactionHash: &felt.One,
			Version:         new(core.TransactionVersion),
		},
		&core.InvokeTransaction{
			TransactionHash: felt.NewFromUint64[felt.Felt](2),
			Version:         new(core.TransactionVersion),
		},
	}
	hash := txs[1].Hash()
	receipts := []*core.TransactionReceipt{{
		TransactionHash: hash,
		ExecutionResources: &core.ExecutionResources{
			TotalGasConsumed: &core.GasConsumed{L1Gas: 7},
		},
	}}
	reader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	reader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil).AnyTimes()
	reader.EXPECT().BlockByHash(header.Hash).
		Return(&core.Block{Header: header, Transactions: txs, Receipts: receipts}, nil).AnyTimes()
	reader.EXPECT().BlockNumberAndIndexByTxHash((*felt.TransactionHash)(hash)).
		Return(header.Number, uint64(1), nil).AnyTimes()
	reader.EXPECT().BlockHeaderByNumber(header.Number).Return(header, nil).AnyTimes()
	var record *tracecache.BlockTrace
	var err error
	if feeder {
		header.ProtocolVersion = "0.12.0"
		invocation := &starknet.FunctionInvocation{
			InternalCalls: make([]starknet.FunctionInvocation, calls),
		}
		result := starknet.BlockTrace{Traces: []starknet.TransactionTrace{
			{TransactionHash: *txs[0].Hash(), ValidateInvocation: invocation},
			{TransactionHash: *hash},
		}}
		record, err = tracecache.FromFeeder(
			[]vm.TransactionType{vm.TxnInvoke, vm.TxnInvoke},
			receipts,
			&result,
		)
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
	}
	require.NoError(t, err)
	cache := tracecache.New[felt.Felt, *tracecache.BlockTrace](1)
	logger := log.NewNopZapLogger()
	h8 := rpcv8.New(reader, nil, nil, logger).WithTraceCache(cache)
	h9 := rpcv9.New(reader, nil, nil, logger).WithTraceCache(cache)
	h10 := rpcv10.New(reader, nil, nil, logger).WithTraceCache(cache)
	_, lease, err := cache.Acquire(t.Context(), header.Hash)
	require.NoError(t, err)
	lease.Publish(record)
	return func() {
		switch version {
		case 8:
			trace, _, rpcErr := h8.TraceTransaction(t.Context(), *hash)
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(7), trace.ExecutionResources.L1Gas)
		case 9:
			trace, _, rpcErr := h9.TraceTransaction(t.Context(), (*felt.TransactionHash)(hash))
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(7), trace.ExecutionResources.L1Gas)
		case 10:
			trace, _, rpcErr := h10.TraceTransaction(t.Context(), (*felt.TransactionHash)(hash))
			require.Nil(t, rpcErr)
			require.Equal(t, uint64(7), trace.ExecutionResources.L1Gas)
		}
	}
}
