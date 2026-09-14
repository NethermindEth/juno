package tracecache

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestVMResultValidationAndReadCoverage(t *testing.T) {
	txs := []core.Transaction{&core.InvokeTransaction{TransactionHash: &felt.One}}
	valid := vm.ExecutionResults{Traces: []vm.TransactionTrace{{Type: vm.TxnInvoke}}, GasConsumed: []core.GasConsumed{{L2Gas: 7}}, NumSteps: 11}
	block, err := FromVM(txs, &valid, false)
	require.NoError(t, err)
	require.Same(t, &valid.Traces[0], block.Traces[0].VMTrace())
	require.Nil(t, block.Traces[0].FeederTrace())
	require.Equal(t, felt.One, block.Traces[0].Hash)
	require.Equal(t, uint64(7), block.Traces[0].Gas.L2Gas)
	require.True(t, block.Covers(false))
	require.False(t, block.Covers(true))
	_, err = FromVM(txs, &valid, true)
	require.EqualError(t, err, "VM omitted initial reads for block trace")
	valid.InitialReads = &vm.InitialReads{}
	block, err = FromVM(txs, &valid, true)
	require.NoError(t, err)
	require.True(t, block.Covers(true))
	block, err = FromVM(txs, &valid, false)
	require.NoError(t, err)
	require.Nil(t, block.InitialReads)
	for _, test := range []struct {
		name   string
		result vm.ExecutionResults
	}{
		{"trace count", vm.ExecutionResults{}},
		{"gas count", vm.ExecutionResults{Traces: valid.Traces}},
	} {
		t.Run(test.name, func(t *testing.T) { _, err := FromVM(txs, &test.result, false); require.Error(t, err) })
	}
	empty, err := FromVM(nil, &vm.ExecutionResults{}, true)
	require.NoError(t, err)
	require.NotNil(t, empty.Traces)
	require.Empty(t, empty.Traces)
	require.Nil(t, empty.InitialReads)
}

func TestFeederMetadataAndReadCoverage(t *testing.T) {
	source := starknet.BlockTrace{Traces: []starknet.TransactionTrace{{TransactionHash: felt.One}, {TransactionHash: felt.Zero}}}
	receipt := &core.TransactionReceipt{TransactionHash: &felt.One, ExecutionResources: &core.ExecutionResources{TotalGasConsumed: &core.GasConsumed{L1Gas: 5}}}
	block, err := FromFeeder([]vm.TransactionType{vm.TxnDeploy, vm.TxnL1Handler}, []*core.TransactionReceipt{receipt}, &source)
	require.NoError(t, err)
	for i := range block.Traces {
		require.Same(t, &source.Traces[i], block.Traces[i].FeederTrace())
		require.Nil(t, block.Traces[i].VMTrace())
	}
	require.True(t, block.Covers(true))
	require.Nil(t, block.InitialReads)
	require.Equal(t, vm.TxnL1Handler, block.Traces[1].Type)
	require.Equal(t, uint64(5), block.Traces[0].Gas.L1Gas)
	require.Zero(t, block.Traces[1].Gas.L1Gas)
	receipt.ExecutionResources.TotalGasConsumed.L1Gas = 9
	require.Equal(t, uint64(5), block.Traces[0].Gas.L1Gas)
	_, err = FromFeeder(nil, nil, &source)
	require.EqualError(t, err, "mismatched number of txs and traces")
}
