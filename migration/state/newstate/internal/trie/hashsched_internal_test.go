package trie

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	//nolint:staticcheck // the deprecated trie is this migration's input
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
	"github.com/stretchr/testify/require"
)

var errInjectedRead = errors.New("injected read failure")

// failingReader fails every Get from failAt onwards. Used single-goroutine by
// the traversal, so a plain counter is enough.
type failingReader struct {
	db.KeyValueReader
	gets   int
	failAt int
}

func (r *failingReader) Get(key []byte, cb func(value []byte) error) error {
	r.gets++
	if r.gets >= r.failAt {
		return errInjectedRead
	}
	return r.KeyValueReader.Get(key, cb)
}

// TestMigrateTrieWaitsForInFlightHashesOnError pins the invariant that
// migrateTrie never returns while a hash batch is still in flight.
// That could result in potential send on a closed p.work channel
func TestMigrateTrieWaitsForInFlightHashesOnError(t *testing.T) {
	prevBatchSize := parallelHashBatchSize
	parallelHashBatchSize = 1 // dispatch on the first binary node
	t.Cleanup(func() { parallelHashBatchSize = prevBatchSize })

	memDB := memory.New()
	seedThreeLeafTrie(t, memDB)

	descs, err := enumerateTries(memDB)
	require.NoError(t, err)
	var desc TrieDesc
	for _, d := range descs {
		if d.DeprecatedTrieBucket == db.StateTrie {
			desc = d
		}
	}
	require.NotNil(t, desc.RootPath, "state trie was not enumerated")
	// Two binary nodes plus three leaves. failAt below is tied to this count:
	// if the fixture ever changes shape, fix both together.
	require.Equal(t, 5, desc.NodeCount)
	desc.NodeCount = SmallTrieThreshold // force the parallel dispatch path

	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	desc.HashFn = func(a, _ *felt.Felt) felt.Felt {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return *a
	}

	pool := newHashWorkerPool()
	defer pool.close()

	reader := &failingReader{KeyValueReader: memDB, failAt: 5}
	batchSem := semaphore.New(common.IngestorCount*2, func() db.Batch {
		return memDB.NewBatch()
	})
	ing := newIngestor(context.Background(), reader, batchSem, pool)

	outputs := make(chan common.Task, common.IngestorCount)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ing.migrateTrie(&ing.Tasks[0], desc, outputs)
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("hash worker never ran: the fixture no longer reaches the parallel path")
	}

	select {
	case err := <-errCh:
		t.Fatalf("migrateTrie returned while a hash batch was in flight: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, errInjectedRead)
	case <-time.After(5 * time.Second):
		t.Fatal("migrateTrie did not return after the in-flight batch drained")
	}
}

// seedThreeLeafTrie writes a deprecated state trie with three leaves
func seedThreeLeafTrie(t *testing.T, database db.KeyValueStore) {
	t.Helper()

	var midKey, rightKey [32]byte
	midKey[0] = 0x02   // path 01…
	rightKey[0] = 0x04 // path 1…

	keys := []felt.Felt{
		{}, // path 00…
		felt.FromBytes[felt.Felt](midKey[:]),
		felt.FromBytes[felt.Felt](rightKey[:]),
	}

	//nolint:staticcheck // deprecated trie is the migration's input
	txn := database.NewIndexedBatch()
	//nolint:staticcheck // deprecated trie is the migration's input
	tr, err := trie.NewTriePedersen(txn, db.StateTrie.Key(), 251)
	require.NoError(t, err)
	for i := range keys {
		value := felt.FromUint64[felt.Felt](uint64(i + 1))
		_, err := tr.Put(&keys[i], &value)
		require.NoError(t, err)
	}
	require.NoError(t, tr.Commit())
	require.NoError(t, txn.Write())
}
