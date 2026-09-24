package tracecache_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/tracecache"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
)

func TestBlockTraceCovers(t *testing.T) {
	for _, test := range []struct {
		name         string
		block        tracecache.BlockTrace
		withoutReads bool
		withReads    bool
	}{
		{
			name: "incomplete VM block",
			block: tracecache.BlockTrace{
				Traces:       []tracecache.TransactionTrace{{Hash: felt.One}},
				InitialReads: &vm.InitialReads{},
			},
		},
		{
			name: "complete VM block without reads",
			block: tracecache.BlockTrace{
				Complete: true,
				Traces:   []tracecache.TransactionTrace{{Hash: felt.One}},
			},
			withoutReads: true,
		},
		{
			name: "complete VM block with reads",
			block: tracecache.BlockTrace{
				Complete:     true,
				Traces:       []tracecache.TransactionTrace{{Hash: felt.One}},
				InitialReads: &vm.InitialReads{},
			},
			withoutReads: true,
			withReads:    true,
		},
		{
			name:         "empty complete block",
			block:        tracecache.BlockTrace{Complete: true},
			withoutReads: true,
		},
		{
			name: "empty complete block with reads",
			block: tracecache.BlockTrace{
				Complete:     true,
				InitialReads: &vm.InitialReads{},
			},
			withoutReads: true,
			withReads:    true,
		},
		{
			name:  "empty incomplete block",
			block: tracecache.BlockTrace{},
		},
		{
			name:         "complete feeder block allows unavailable reads",
			block:        tracecache.BlockTrace{Complete: true, Source: tracecache.Feeder},
			withoutReads: true,
			withReads:    true,
		},
		{
			name:  "feeder source does not imply completeness",
			block: tracecache.BlockTrace{Source: tracecache.Feeder},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.withoutReads, test.block.Covers(false))
			require.Equal(t, test.withReads, test.block.Covers(true))
		})
	}
}

func TestBlockTraceCoversTarget(t *testing.T) {
	firstHash := felt.NewFromUint64[felt.TransactionHash](1)
	lastHash := felt.NewFromUint64[felt.TransactionHash](2)
	first := &tracecache.TransactionTarget{Index: 0, Hash: firstHash}
	last := &tracecache.TransactionTarget{Index: 1, Hash: lastHash}
	block := tracecache.BlockTrace{
		Traces: []tracecache.TransactionTrace{{Hash: felt.Felt(*firstHash)}},
	}

	require.True(t, block.CoversTarget(first, false))
	require.False(t, block.CoversTarget(last, false), "target is just beyond the cached prefix")
	require.False(t, block.CoversTarget(first, true), "VM reads were not collected")

	block.InitialReads = &vm.InitialReads{}
	require.True(t, block.CoversTarget(first, true))
	require.False(t, block.CoversTarget(last, true), "reads do not extend transaction coverage")

	block.Traces = append(block.Traces, tracecache.TransactionTrace{Hash: felt.Felt(*lastHash)})
	block.Complete = true
	require.True(t, block.CoversTarget(first, false))
	require.True(t, block.CoversTarget(last, false))
	require.True(t, block.CoversTarget(last, true))
	block.InitialReads = nil
	require.False(t, block.CoversTarget(last, true), "completeness does not imply collected reads")

	block.Source = tracecache.Feeder
	require.True(t, block.CoversTarget(last, true), "feeder reads are unavailable")

	empty := tracecache.BlockTrace{}
	require.False(t, empty.CoversTarget(first, false))
}

func TestBlockTraceCoversTargetRequiresTarget(t *testing.T) {
	for _, test := range []struct {
		name     string
		complete bool
	}{
		{"incomplete block", false},
		{"complete block", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			block := tracecache.BlockTrace{Complete: test.complete}
			require.Panics(t, func() {
				block.CoversTarget(nil, false)
			})
		})
	}
}

func TestBlockTraceValidTarget(t *testing.T) {
	firstHash := felt.NewFromUint64[felt.TransactionHash](1)
	lastHash := felt.NewFromUint64[felt.TransactionHash](2)
	block := tracecache.BlockTrace{
		Traces: []tracecache.TransactionTrace{
			{Hash: felt.Felt(*firstHash)},
			{Hash: felt.Felt(*lastHash)},
		},
	}
	for _, test := range []struct {
		name   string
		target *tracecache.TransactionTarget
		valid  bool
	}{
		{"whole block", nil, true},
		{"first transaction", &tracecache.TransactionTarget{Index: 0, Hash: firstHash}, true},
		{"last transaction", &tracecache.TransactionTarget{Index: 1, Hash: lastHash}, true},
		{"wrong hash", &tracecache.TransactionTarget{Index: 0, Hash: lastHash}, false},
		{"missing hash", &tracecache.TransactionTarget{Index: 0}, false},
		{"out of range", &tracecache.TransactionTarget{Index: 2, Hash: lastHash}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.valid, block.ValidTarget(test.target))
			complete := block
			complete.Complete = true
			require.Equal(t, test.valid, complete.ValidTarget(test.target))
		})
	}
	empty := tracecache.BlockTrace{}
	require.True(t, empty.ValidTarget(nil))
	require.False(t, empty.ValidTarget(&tracecache.TransactionTarget{Hash: firstHash}))
}

func TestTransactionTraceSourceAccessors(t *testing.T) {
	t.Run("VM", func(t *testing.T) {
		txs := []core.Transaction{&core.InvokeTransaction{TransactionHash: &felt.One}}
		result := vm.ExecutionResults{
			Traces:      []vm.TransactionTrace{{Type: vm.TxnInvoke}},
			GasConsumed: []core.GasConsumed{{}},
		}
		block, err := tracecache.FromVM(txs, &result, false)
		require.NoError(t, err)
		require.Same(t, &result.Traces[0], block.Traces[0].VMTrace())
		require.Nil(t, block.Traces[0].FeederTrace())
	})
	t.Run("feeder", func(t *testing.T) {
		source := starknet.BlockTrace{Traces: []starknet.TransactionTrace{
			{TransactionHash: felt.One},
			{TransactionHash: felt.Zero},
		}}
		block, err := tracecache.FromFeeder(
			[]vm.TransactionType{vm.TxnDeploy, vm.TxnL1Handler},
			nil,
			&source,
		)
		require.NoError(t, err)
		for i := range block.Traces {
			require.Same(t, &source.Traces[i], block.Traces[i].FeederTrace())
			require.Nil(t, block.Traces[i].VMTrace())
		}
	})
}
