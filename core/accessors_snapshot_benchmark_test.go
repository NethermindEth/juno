package core_test

import (
	"math/rand/v2"
	"os"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/require"
)

// snapshotDBEnv points at a synced node's database directory. The benchmarks below are skipped
// unless it is set, and they only read, but pebble replays the WAL and may compact on open, so
// point it at a copy rather than at a snapshot worth keeping.
const snapshotDBEnv = "JUNO_BENCH_DB"

// Each arm draws from its own half of the pool so neither warms the other's blocks. Size it for
// iterations × -count per arm, so repeat rounds keep reading hashes that have not been read yet.
const snapshotSampleCount = 48000

func openSnapshotDB(b *testing.B) db.KeyValueStore {
	b.Helper()
	path := os.Getenv(snapshotDBEnv)
	if path == "" {
		b.Skipf("set %s to a node database directory to run this benchmark", snapshotDBEnv)
	}

	database, err := pebblev2.New(path, pebblev2.WithLogger(quietPebbleLogger{}))
	require.NoError(b, err)
	return database
}

// sampleTransactionHashes walks the transaction-hash index and keeps every nth key whose block is
// readable. Index order is hash order, so the sample spreads across all heights; a synced node can
// still hold index entries whose block was pruned or reverted, hence the read-back.
func sampleTransactionHashes(b *testing.B, database db.KeyValueStore) []*felt.TransactionHash {
	b.Helper()
	it, err := database.NewIterator(db.TransactionBlockNumbersAndIndicesByHash.Key(), true)
	require.NoError(b, err)
	defer func() { require.NoError(b, it.Close()) }()

	rng := rand.New(rand.NewPCG(42, 0))
	hashes := make([]*felt.TransactionHash, 0, snapshotSampleCount)
	const prefixLen = 1
	for it.Next() && len(hashes) < snapshotSampleCount {
		if rng.IntN(4) != 0 {
			continue
		}
		key := it.Key()
		if len(key) != prefixLen+felt.Bytes {
			continue
		}
		var hash felt.TransactionHash
		(*felt.Felt)(&hash).SetBytes(key[prefixLen:])

		at, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(database, &hash)
		if err != nil {
			continue
		}
		if _, _, err := core.GetTransactionAndReceiptByBlockAndIndex(
			database, at.Number, at.Index,
		); err != nil {
			continue
		}
		if _, err := core.GetBlockHeaderHashByNumber(database, at.Number); err != nil {
			continue
		}
		hashes = append(hashes, &hash)
	}
	require.NotEmpty(b, hashes)
	return hashes
}

// BenchmarkReceiptByHashReadsSnapshot replays the database reads starknet_getTransactionReceipt
// makes, against a real node database at random heights.
//
// Selecting the sample reads the blocks it selects, so the database is reopened afterwards: pebble
// caches decompressed blocks and open table readers, and both are dropped on close. The timed run
// therefore pays decompression, cache allocation, table opens and multi-level probing — the costs
// the fixture benchmark cannot show. The host page cache still holds the files, so disk I/O is
// cheaper here than on a node whose working set exceeds memory.
func BenchmarkReceiptByHashReadsSnapshot(b *testing.B) {
	setupDB := openSnapshotDB(b)
	hashes := sampleTransactionHashes(b, setupDB)
	require.NoError(b, setupDB.Close())

	database := openSnapshotDB(b)
	b.Cleanup(func() { require.NoError(b, database.Close()) })

	// Disjoint halves: whichever arm runs first must not warm the other's blocks. The cursors are
	// declared out here so repeat runs under -count keep advancing into unread hashes instead of
	// replaying the warm prefix.
	beforeHashes, afterHashes := hashes[:len(hashes)/2], hashes[len(hashes)/2:]
	beforeAt, afterAt := 0, 0

	// before: hash index, transaction, receipt, full header.
	b.Run("before", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			hash := beforeHashes[beforeAt%len(beforeHashes)]
			beforeAt++

			at, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(database, hash)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := core.GetTransactionByBlockAndIndex(
				database, at.Number, at.Index,
			); err != nil {
				b.Fatal(err)
			}
			if _, err := core.GetReceiptByBlockAndIndex(database, at.Number, at.Index); err != nil {
				b.Fatal(err)
			}
			if _, err := core.GetBlockHeaderByNumber(database, at.Number); err != nil {
				b.Fatal(err)
			}
		}
	})

	// after: transaction and receipt share one read, and the block hash is projected out of the
	// header rather than decoding all of it.
	b.Run("after", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			hash := afterHashes[afterAt%len(afterHashes)]
			afterAt++

			at, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(database, hash)
			if err != nil {
				b.Fatal(err)
			}
			if _, _, err := core.GetTransactionAndReceiptByBlockAndIndex(
				database, at.Number, at.Index,
			); err != nil {
				b.Fatal(err)
			}
			if _, err := core.GetBlockHeaderHashByNumber(database, at.Number); err != nil {
				b.Fatal(err)
			}
		}
	})
}
