package trie

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_FreshDBIsNoOp(t *testing.T) {
	testDB := memory.New()
	t.Cleanup(func() { testDB.Close() })

	state, err := (&Migrator{}).Migrate(context.Background(), testDB, nil, nopLogger())
	require.NoError(t, err)
	assert.Nil(
		t,
		state,
		"fresh DB must mark migration applied (nil intermediate state) without doing work",
	)
}

func TestMigrate_RunsWhenOldDataPresent(t *testing.T) {
	leaves := randomLeaves(100, 7)
	testDB, _, _, _, _ := buildFullDB(t, leaves)

	needed, err := needsMigration(testDB)
	require.NoError(t, err)
	require.True(t, needed, "precondition: DB has old-format data")

	state, err := (&Migrator{}).Migrate(context.Background(), testDB, nil, nopLogger())
	require.NoError(t, err)
	assert.Nil(t, state, "completed migration must return nil intermediate state")

	stillNeeded, err := needsMigration(testDB)
	require.NoError(t, err)
	assert.False(t, stillNeeded, "old-format buckets should be empty after migration")
}

func TestMigrationIsResumable(t *testing.T) {
	leaves := randomLeaves(1000, 42)

	// Reference: full migration from scratch.
	refDB, _, _, _, _ := buildFullDB(t, leaves)
	_, err := runMigration(context.Background(), refDB, nopLogger())
	require.NoError(t, err)

	// Partial DB: both tries in old format initially.
	partialDB, _, _, _, _ := buildFullDB(t, leaves)

	// Manually migrate only the class trie to simulate a mid-run interruption.
	classPrefix := db.ClassesTrie.Key()
	var classRootPath *trie.BitArray
	require.NoError(t, partialDB.Get(classPrefix, func(val []byte) error {
		classRootPath = parseRootPath(val)
		return nil
	}))
	classDesc := TrieDesc{
		OldBucket: db.ClassesTrie,
		NewBucket: db.ClassTrie,
		HashFn:    crypto.Poseidon,
		NodeCount: len(leaves),
		RootPath:  classRootPath,
	}
	batch := partialDB.NewBatch()

	pool := newHashWorkerPool()
	defer pool.close()
	migrator := newDFSMigrator(true, pool)
	stack := make([]dfsFrame, 0, dfsStackCap)
	_, _, err = migrator.Migrate(partialDB, batch, classDesc, noFlush, stack)
	require.NoError(t, err)
	require.NoError(t, batch.Write())

	done, err := hasDestRoot(partialDB, db.ClassTrie, &classDesc.Owner)
	require.NoError(t, err)
	assert.True(t, done, "class trie destination root should be present after partial migration")

	needed, err := needsMigration(partialDB)
	require.NoError(t, err)
	assert.True(t, needed, "migration should still be needed after partial work")

	// enumerateTries yields every discovered trie unfiltered; the
	// already-done short-circuit lives in the ingestor now. We assert that
	// the class trie *is* yielded here, and rely on the end-to-end equality
	// check below to confirm the ingestor's idempotency guard skips it.
	descsRemaining := collectTries(t, partialDB)
	foundClass := false
	for _, d := range descsRemaining {
		if d.OldBucket == db.ClassesTrie {
			foundClass = true
		}
	}
	assert.True(
		t,
		foundClass,
		"enumerateTries should yield class trie even when destination root is present",
	)

	// Resume: migration completes the remaining contract trie.
	_, err = runMigration(context.Background(), partialDB, nopLogger())
	require.NoError(t, err)

	// Final state must match the reference full-run output for every new-format bucket.
	for _, bucket := range []db.Bucket{db.ClassTrie, db.ContractTrieContract} {
		refKeys := allKeysUnder(t, refDB, bucket)
		resumedKeys := allKeysUnder(t, partialDB, bucket)
		assert.Equal(t, refKeys, resumedKeys,
			"resumed migration result differs from full run for bucket %v", bucket)
	}
}

// buildFullDB creates an old-format DB with class, contract, and one storage trie,
// all populated with the same leaf set. Returns the DB and the old-format root hashes.
func buildFullDB(t *testing.T, leaves leafMap) (
	database db.KeyValueStore,
	classRoot felt.Felt,
	contractRoot felt.Felt,
	storageRoot felt.Felt,
	owner felt.Address,
) {
	t.Helper()
	database = memory.New()

	classRoot = buildDeprecatedTrie(t, database, leaves, trie.NewTriePoseidon, db.ClassesTrie.Key())
	contractRoot = buildDeprecatedTrie(t, database, leaves, trie.NewTriePedersen, db.StateTrie.Key())

	var ownerFelt felt.Felt
	ownerFelt.SetUint64(42)
	owner = felt.Address(ownerFelt)
	ownerBytes := ownerFelt.Bytes()
	storagePrefix := db.ContractStorage.Key(ownerBytes[:])
	storageRoot = buildDeprecatedTrie(t, database, leaves, trie.NewTriePedersen, storagePrefix)

	return database, classRoot, contractRoot, storageRoot, owner
}

func insertFakeStorageNodes(t *testing.T, database db.KeyValueStore, owner felt.Address, n int) {
	t.Helper()
	ownerFelt := felt.Felt(owner)
	ownerBytes := ownerFelt.Bytes()
	ownerPrefix := db.ContractStorage.Key(ownerBytes[:])

	require.NoError(t, database.Put(ownerPrefix, []byte{0}))

	for i := range n {
		key := append(bytes.Clone(ownerPrefix), 8, byte(i))
		require.NoError(t, database.Put(key, []byte{0xFF}))
	}
}

func collectTries(t *testing.T, r db.KeyValueReader) []TrieDesc {
	t.Helper()
	descs, err := enumerateTries(r)
	require.NoError(t, err)
	return descs
}

func TestEnumerateTries_EmptyDBYieldsClassAndContractTries(t *testing.T) {
	testDB := memory.New()
	descs := collectTries(t, testDB)
	require.Len(t, descs, 2)
	assert.Equal(t, db.ClassesTrie, descs[0].OldBucket)
	assert.Equal(t, db.StateTrie, descs[1].OldBucket)
}

func TestEnumerateTries_GlobalTriesPresent(t *testing.T) {
	testDB := memory.New()
	descs := collectTries(t, testDB)
	hasClass := slices.ContainsFunc(descs, func(d TrieDesc) bool {
		return d.OldBucket == db.ClassesTrie && d.NewBucket == db.ClassTrie
	})
	hasContract := slices.ContainsFunc(descs, func(d TrieDesc) bool {
		return d.OldBucket == db.StateTrie && d.NewBucket == db.ContractTrieContract
	})
	assert.True(t, hasClass)
	assert.True(t, hasContract)
}

func TestEnumerateTries_StorageTriesDiscovered(t *testing.T) {
	testDB := memory.New()
	var owners [3]felt.Address
	for i := range owners {
		var f felt.Felt
		f.SetUint64(uint64(i + 1))
		owners[i] = felt.Address(f)
		insertFakeStorageNodes(t, testDB, owners[i], 5)
	}

	descs := collectTries(t, testDB)
	require.Len(t, descs, 5)

	storageCount := 0
	for _, d := range descs {
		if d.OldBucket == db.ContractStorage {
			assert.Equal(t, db.ContractTrieStorage, d.NewBucket)
			storageCount++
		}
	}
	assert.Equal(t, 3, storageCount)
}

func TestEnumerateTries_NodeCountIsCorrect(t *testing.T) {
	testDB := memory.New()
	var ownerFelt felt.Felt
	ownerFelt.SetUint64(99)
	owner := felt.Address(ownerFelt)
	insertFakeStorageNodes(t, testDB, owner, 7)

	descs := collectTries(t, testDB)
	require.Len(t, descs, 3)

	idx := slices.IndexFunc(descs, func(d TrieDesc) bool { return d.OldBucket == db.ContractStorage })
	require.NotEqual(t, -1, idx)
	assert.Equal(t, 7, descs[idx].NodeCount)
}

func TestEnumerateTries_StorageTrieCountsPresent(t *testing.T) {
	testDB := memory.New()
	for i, n := range []int{3, 7, 1} {
		var f felt.Felt
		f.SetUint64(uint64(i + 1))
		insertFakeStorageNodes(t, testDB, felt.Address(f), n)
	}

	descs := collectTries(t, testDB)
	var storageCounts []int
	for _, d := range descs {
		if d.OldBucket == db.ContractStorage {
			storageCounts = append(storageCounts, d.NodeCount)
		}
	}
	slices.Sort(storageCounts)
	assert.Equal(t, []int{1, 3, 7}, storageCounts)
}

func TestEnumerateTries_StorageTrieOwnerMatchesKey(t *testing.T) {
	testDB := memory.New()
	var ownerFelt felt.Felt
	ownerFelt.SetUint64(12345)
	owner := felt.Address(ownerFelt)
	insertFakeStorageNodes(t, testDB, owner, 3)

	descs := collectTries(t, testDB)
	require.Len(t, descs, 3)

	idx := slices.IndexFunc(descs, func(d TrieDesc) bool { return d.OldBucket == db.ContractStorage })
	require.NotEqual(t, -1, idx)
	assert.Equal(t, owner, descs[idx].Owner)
}

func TestEnumerateTries_MultipleOwnersOrdered(t *testing.T) {
	testDB := memory.New()
	owners := make([]felt.Address, 5)
	for i := range owners {
		var f felt.Felt
		f.SetUint64(uint64(i + 1))
		owners[i] = felt.Address(f)
		insertFakeStorageNodes(t, testDB, owners[i], i+1)
	}

	descs := collectTries(t, testDB)
	require.Len(t, descs, 7)

	var storageCounts []int
	for _, d := range descs {
		if d.OldBucket == db.ContractStorage {
			storageCounts = append(storageCounts, d.NodeCount)
		}
	}
	slices.Sort(storageCounts)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, storageCounts)
}
