package statedifflength

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/require"
)

// discardWriter stands in for the migration's batch so a benchmark measures the
// per-block work rather than the growth of the batch buffer.
type discardWriter struct{}

func (discardWriter) Put(_, _ []byte) error { return nil }
func (discardWriter) Delete(_ []byte) error { return nil }

// TestReadCommitmentsMatchesAccessor pins the worker's read path to the core
// accessor it replaces, so the two cannot drift apart on the bucket or the encoding.
func TestReadCommitmentsMatchesAccessor(t *testing.T) {
	one := felt.NewFromUint64[felt.Felt](1)
	records := map[string]*core.BlockCommitments{
		"all fields": {
			TransactionCommitment: felt.NewFromUint64[felt.Felt](11),
			EventCommitment:       felt.NewFromUint64[felt.Felt](22),
			ReceiptCommitment:     felt.NewFromUint64[felt.Felt](33),
			StateDiffCommitment:   felt.NewFromUint64[felt.Felt](44),
			StateDiffLength:       55,
		},
		"some nil felts": {TransactionCommitment: one, ReceiptCommitment: one},
		"no felts":       {StateDiffLength: 7},
		"nothing set":    {},
		"large length":   {TransactionCommitment: one, StateDiffLength: 1 << 40},
		"zeroed felt":    {TransactionCommitment: felt.NewFromUint64[felt.Felt](0)},
	}

	for name, record := range records {
		t.Run(name, func(t *testing.T) {
			database := memory.New()
			require.NoError(t, core.WriteBlockCommitment(database, 9, record))

			expected, err := core.GetBlockCommitmentByBlockNum(database, 9)
			require.NoError(t, err)

			var reader worker
			require.NoError(t, reader.readCommitments(database, 9))
			require.Equal(t, expected, &reader.commitments)
		})
	}
}

func TestReadCommitmentsReportsMissingRecord(t *testing.T) {
	var reader worker
	err := reader.readCommitments(memory.New(), 3)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}

// TestReadStateDiffLengthMatchesAccessor pins the read path the migration actually
// takes to the one it replaced — reading the state update through core and calling
// core.StateDiff.Length() — over blocks whose diffs cover every counted field. The
// walker is checked against the decoder on raw bytes elsewhere; this checks that it
// is wired to the same record.
func TestReadStateDiffLengthMatchesAccessor(t *testing.T) {
	database := memory.New()

	const blocks = 50
	source := rand.New(rand.NewPCG(31, 41))
	for blockNumber := range uint64(blocks) {
		stateUpdate := randomStateUpdate(source, 20)
		require.NoError(t, core.WriteStateUpdateByBlockNum(database, blockNumber, stateUpdate))
	}

	var covered uint64
	for blockNumber := range uint64(blocks) {
		stateUpdate, err := core.GetStateUpdateByBlockNum(database, blockNumber)
		require.NoError(t, err)
		expected := stateUpdate.StateDiff.Length()

		length, err := readStateDiffLength(database, blockNumber)
		require.NoError(t, err)
		require.Equal(t, expected, length, "block %d", blockNumber)

		covered += expected
	}
	require.Greater(t, covered, uint64(500), "the blocks carry too few entries to prove anything")
}

func TestReadStateDiffLengthReportsMissingRecord(t *testing.T) {
	_, err := readStateDiffLength(memory.New(), 3)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}

// TestBackfillBlockReusesWithoutBleeding is the guard on the reuse: one worker
// processes blocks whose commitments differ in every field, and each stored record
// must come back exactly as it went in apart from the backfilled length. A field
// carried over from the previous block would show up here, since the decoder leaves
// a field absent from a record untouched.
func TestBackfillBlockReusesWithoutBleeding(t *testing.T) {
	one := felt.NewFromUint64[felt.Felt](1)
	blocks := []*core.BlockCommitments{
		{
			TransactionCommitment: felt.NewFromUint64[felt.Felt](11),
			EventCommitment:       felt.NewFromUint64[felt.Felt](22),
			ReceiptCommitment:     felt.NewFromUint64[felt.Felt](33),
			StateDiffCommitment:   felt.NewFromUint64[felt.Felt](44),
		},
		// Every felt nil: none may survive from block 0.
		{},
		// A length already set: it must be replaced, not kept or added to.
		{TransactionCommitment: one, StateDiffLength: 999},
		{EventCommitment: one},
		{
			TransactionCommitment: felt.NewFromUint64[felt.Felt](66),
			StateDiffCommitment:   felt.NewFromUint64[felt.Felt](77),
		},
	}

	database := memory.New()
	for blockNumber, commitments := range blocks {
		require.NoError(t, core.WriteBlockCommitment(database, uint64(blockNumber), commitments))
		// Block n gets n+1 storage diff entries, so a length carried over is visible.
		benchmarkBlock(t, database, uint64(blockNumber), uint64(blockNumber)+1)
	}

	batch := database.NewBatch()
	var reader worker
	for blockNumber := range blocks {
		require.NoError(t, reader.backfillBlock(database, batch, uint64(blockNumber)))
	}
	require.NoError(t, batch.Write())

	for blockNumber, original := range blocks {
		stored, err := core.GetBlockCommitmentByBlockNum(database, uint64(blockNumber))
		require.NoError(t, err)

		expected := *original
		expected.StateDiffLength = uint64(blockNumber) + 1
		require.Equal(t, &expected, stored, "block %d", blockNumber)
	}
}

// TestReadCommitmentsReusesFelts pins the point of the exercise: after the first
// block, decoding a commitments record allocates nothing but the lookup key.
func TestReadCommitmentsReusesFelts(t *testing.T) {
	database := memory.New()
	require.NoError(t, core.WriteBlockCommitment(database, 1, &core.BlockCommitments{
		TransactionCommitment: felt.NewFromUint64[felt.Felt](11),
		EventCommitment:       felt.NewFromUint64[felt.Felt](22),
		ReceiptCommitment:     felt.NewFromUint64[felt.Felt](33),
		StateDiffCommitment:   felt.NewFromUint64[felt.Felt](44),
	}))

	var reader worker
	require.NoError(t, reader.readCommitments(database, 1)) // warm up, then measure
	reused := reader.commitments.TransactionCommitment

	allocations := testing.AllocsPerRun(100, func() {
		if err := reader.readCommitments(database, 1); err != nil {
			t.Fatal(err)
		}
	})
	// Only db.BlockCommitmentsKey still allocates; the struct and its four felts do not.
	require.LessOrEqual(t, allocations, float64(2))
	require.Same(t, reused, reader.commitments.TransactionCommitment,
		"the felt should be decoded into, not replaced")
}

// benchmarkBlock stores a block whose state diff holds roughly entries entries.
func benchmarkBlock(tb testing.TB, database db.KeyValueStore, blockNumber, entries uint64) {
	tb.Helper()

	source := rand.New(rand.NewPCG(blockNumber, 99))
	const entriesPerContract = 8
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for range entries / entriesPerContract {
		storage := make(map[felt.Felt]*felt.Felt, entriesPerContract)
		for range entriesPerContract {
			storage[*randomFelt(source)] = randomFelt(source)
		}
		storageDiffs[*randomFelt(source)] = storage
	}
	nonces := make(map[felt.Felt]*felt.Felt)
	for range entries % entriesPerContract {
		nonces[*randomFelt(source)] = randomFelt(source)
	}

	require.NoError(tb, core.WriteStateUpdateByBlockNum(database, blockNumber, &core.StateUpdate{
		BlockHash: randomFelt(source),
		NewRoot:   randomFelt(source),
		OldRoot:   randomFelt(source),
		StateDiff: &core.StateDiff{StorageDiffs: storageDiffs, Nonces: nonces},
	}))
}

// backfillBlockByDecoding is the implementation this package replaced: it decodes the
// whole state update to call Length() on it, and allocates a commitments record per
// block. It is kept here, out of the production path, purely so
// BenchmarkBackfillBlock can show what the change is worth.
func backfillBlockByDecoding(r db.KeyValueReader, batch db.KeyValueWriter, blockNumber uint64) error {
	commitments, err := core.GetBlockCommitmentByBlockNum(r, blockNumber)
	if err != nil {
		return fmt.Errorf("getting commitments for block %d: %w", blockNumber, err)
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(r, blockNumber)
	if err != nil {
		return fmt.Errorf("getting state update for block %d: %w", blockNumber, err)
	}

	commitments.StateDiffLength = stateUpdate.StateDiff.Length()
	return core.WriteBlockCommitment(batch, blockNumber, commitments)
}

// TestBackfillBlockMatchesDecodingImplementation keeps the benchmark honest: the
// baseline it compares against must still produce the same records as the current
// path, or the comparison is between two different jobs.
func TestBackfillBlockMatchesDecodingImplementation(t *testing.T) {
	database := memory.New()
	source := rand.New(rand.NewPCG(17, 19))

	const blocks = 20
	for blockNumber := range uint64(blocks) {
		require.NoError(t, core.WriteStateUpdateByBlockNum(
			database, blockNumber, randomStateUpdate(source, 15),
		))
		require.NoError(t, core.WriteBlockCommitment(database, blockNumber, &core.BlockCommitments{
			TransactionCommitment: randomFelt(source),
			EventCommitment:       randomFelt(source),
		}))
	}

	// Each path writes to its own database so the records can be compared as stored.
	fromWalking, fromDecoding := memory.New(), memory.New()
	var reader worker
	for blockNumber := range uint64(blocks) {
		require.NoError(t, reader.backfillBlock(database, fromWalking, blockNumber))
		require.NoError(t, backfillBlockByDecoding(database, fromDecoding, blockNumber))

		require.Equal(t,
			storedCommitments(t, fromDecoding, blockNumber),
			storedCommitments(t, fromWalking, blockNumber),
			"block %d must be written byte for byte the same", blockNumber,
		)
	}
}

// storedCommitments returns the block's commitments record exactly as stored.
func storedCommitments(t *testing.T, r db.KeyValueReader, blockNumber uint64) []byte {
	t.Helper()

	var record []byte
	require.NoError(t, r.Get(db.BlockCommitmentsKey(blockNumber), func(data []byte) error {
		record = slices.Clone(data)
		return nil
	}))
	return record
}

// BenchmarkBackfillBlock measures the whole per-block path the migration runs
// against the implementation it replaced, for diffs from a quiet block up to a busy
// mainnet one. It runs against an in-memory database, so it reports the CPU and
// allocation cost only — a real migration also pays for reading each state update
// off disk, which neither path avoids.
func BenchmarkBackfillBlock(b *testing.B) {
	one := felt.NewFromUint64[felt.Felt](1)
	for _, entries := range []uint64{10, 100, 1000, 5000} {
		database := memory.New()
		benchmarkBlock(b, database, 0, entries)
		require.NoError(b, core.WriteBlockCommitment(database, 0, &core.BlockCommitments{
			TransactionCommitment: one,
			EventCommitment:       one,
			ReceiptCommitment:     one,
			StateDiffCommitment:   one,
		}))

		b.Run(fmt.Sprintf("entries=%d/decode", entries), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := backfillBlockByDecoding(database, discardWriter{}, 0); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("entries=%d/walk", entries), func(b *testing.B) {
			var reader worker
			b.ReportAllocs()
			for b.Loop() {
				if err := reader.backfillBlock(database, discardWriter{}, 0); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
