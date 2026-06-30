package walstore_test

import (
	"sort"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	consensuswal "github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/consensus/walstore"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

const benchmarkRound = types.Round(1)

func BenchmarkTendermintWALStoreConsensusEntryDurability(b *testing.B) {
	allEntryTypes := makeWALEntriesForHeight(1)
	stages := []struct {
		name  string
		entry starknet.WALEntry
	}{
		{"Proposal", allEntryTypes[0]},
		{"Prevote", allEntryTypes[1]},
		{"Precommit", allEntryTypes[2]},
		{"Timeout", allEntryTypes[3]},
	}

	for _, stage := range stages {
		b.Run(stage.name, func(b *testing.B) {
			walStore := newBenchmarkWALStore(b)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				require.NoError(b, walStore.SetWALEntry(stage.entry))
				require.NoError(b, walStore.Flush())
			}
		})
	}

	b.Run("Height", func(b *testing.B) {
		walStore := newBenchmarkWALStore(b)
		flushGroups := makeHeightWALFlushGroups(1)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			writeWALFlushGroups(b, walStore, flushGroups)
		}
	})

	b.Run("HeightWithCleanup", func(b *testing.B) {
		walStore := newBenchmarkWALStore(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := range b.N {
			height := types.Height(i + 1)
			writeWALFlushGroups(b, walStore, makeHeightWALFlushGroups(height))

			b.StopTimer()
			walstore.ForcePendingCleanup(walStore)
			b.StartTimer()

			require.NoError(b, walStore.DeleteWALEntries(height))
			require.NoError(b, walStore.Flush())
		}
	})
}

// Measures per-height latency with cleanup at the real prune interval.
func BenchmarkTendermintWALStoreSustainedHeightLatency(b *testing.B) {
	const heights = 1000

	walStore := newBenchmarkWALStore(b)
	flushGroupsByHeight := make([][][]starknet.WALEntry, heights)
	for i := range flushGroupsByHeight {
		flushGroupsByHeight[i] = makeHeightWALFlushGroups(types.Height(i + 1))
	}

	latencies := make([]time.Duration, 0, heights)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range heights {
		height := types.Height(i + 1)
		start := time.Now()
		writeWALFlushGroups(b, walStore, flushGroupsByHeight[i])
		require.NoError(b, walStore.DeleteWALEntries(height))
		require.NoError(b, walStore.Flush())
		latencies = append(latencies, time.Since(start))
	}
	b.StopTimer()

	reportHeightLatency(b, latencies)
}

func reportHeightLatency(b *testing.B, latencies []time.Duration) {
	b.Helper()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	pct := func(p float64) time.Duration {
		idx := int(p * float64(len(latencies)-1))
		return latencies[idx]
	}

	b.Logf(
		"n=%d p50=%s p95=%s p99=%s max=%s",
		len(latencies),
		pct(0.50),
		pct(0.95),
		pct(0.99),
		latencies[len(latencies)-1],
	)
}

func newBenchmarkWALStore(b *testing.B) testTendermintWALStore {
	b.Helper()
	walStore, testDB, _ := newTestTendermintWALStore(b)
	b.Cleanup(func() {
		require.NoError(b, walStore.Close())
		require.NoError(b, testDB.Close())
	})
	return walStore
}

func BenchmarkTendermintWALStoreReplay(b *testing.B) {
	walStore, testDB, dbPath := newTestTendermintWALStore(b)
	writeWALFlushGroups(b, walStore, makeHeightWALFlushGroups(1))
	require.NoError(b, walStore.Close())
	require.NoError(b, testDB.Close())

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		ws, db := openTestTendermintWALStore(b, dbPath)

		b.StopTimer()
		require.NoError(b, ws.Close())
		require.NoError(b, db.Close())
		b.StartTimer()
	}
}

func writeWALFlushGroups(
	tb testing.TB,
	walStore testTendermintWALStore,
	flushGroups [][]starknet.WALEntry,
) {
	for _, group := range flushGroups {
		for _, entry := range group {
			require.NoError(tb, walStore.SetWALEntry(entry))
		}
		require.NoError(tb, walStore.Flush())
	}
}

// makeHeightWALFlushGroups returns one height of WAL writes for 4 validators.
// Each group is flushed together, matching driver execution.
func makeHeightWALFlushGroups(height types.Height) [][]starknet.WALEntry {
	start := consensuswal.Start(height)
	proposer := felt.FromUint64[starknet.Address](1)
	proposalValue := felt.FromUint64[starknet.Value](10)
	proposalHash := proposalValue.Hash()
	proposal := starknet.WALProposal{
		MessageHeader: starknet.MessageHeader{Height: height, Round: benchmarkRound, Sender: proposer},
		ValidRound:    benchmarkRound,
		Value:         &proposalValue,
	}

	prevote1 := makeBenchmarkWALPrevote(height, 1, &proposalHash)
	prevote2 := makeBenchmarkWALPrevote(height, 2, &proposalHash)
	prevote3 := makeBenchmarkWALPrevote(height, 3, &proposalHash)
	precommit1 := makeBenchmarkWALPrecommit(height, 1, &proposalHash)
	precommit2 := makeBenchmarkWALPrecommit(height, 2, &proposalHash)
	precommit3 := makeBenchmarkWALPrecommit(height, 3, &proposalHash)

	return [][]starknet.WALEntry{
		{&start, &proposal},
		{&prevote1},
		{&prevote2},
		{&prevote3},
		{&precommit1},
		{&precommit2},
		{&precommit3},
	}
}

func makeBenchmarkWALPrevote(
	height types.Height,
	sender uint64,
	id *starknet.Hash,
) starknet.WALPrevote {
	return starknet.WALPrevote{
		MessageHeader: starknet.MessageHeader{
			Height: height,
			Round:  benchmarkRound,
			Sender: felt.FromUint64[starknet.Address](sender),
		},
		ID: id,
	}
}

func makeBenchmarkWALPrecommit(
	height types.Height,
	sender uint64,
	id *starknet.Hash,
) starknet.WALPrecommit {
	return starknet.WALPrecommit{
		MessageHeader: starknet.MessageHeader{
			Height: height,
			Round:  benchmarkRound,
			Sender: felt.FromUint64[starknet.Address](sender),
		},
		ID: id,
	}
}
