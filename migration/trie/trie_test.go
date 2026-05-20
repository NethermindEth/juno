package trie_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/triedb/rawdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	trielib "github.com/NethermindEth/juno/migration/trie"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type leafMap map[felt.Felt]felt.Felt

type trieCase struct {
	name           string
	oldBucket      db.Bucket
	newBucket      db.Bucket
	owner          felt.Address
	oldBuildPrefix func(owner *felt.Address) []byte
	newTrieID      func(owner *felt.Address) trieutils.TrieID
	hashFn         crypto.HashFn
	//nolint:staticcheck // Necessary for old state
	buildOldFn func(db.IndexedBatch, []byte, uint8) (*trie.Trie, error)
}

var trieCases = []trieCase{
	{
		name:           "ClassTrie",
		oldBucket:      db.ClassesTrie,
		newBucket:      db.ClassTrie,
		oldBuildPrefix: func(_ *felt.Address) []byte { return []byte{byte(db.ClassesTrie)} },
		newTrieID: func(_ *felt.Address) trieutils.TrieID {
			return trieutils.NewClassTrieID(felt.StateRootHash(felt.One))
		},
		hashFn:     crypto.Poseidon,
		buildOldFn: trie.NewTriePoseidon,
	},
	{
		name:           "ContractTrie",
		oldBucket:      db.StateTrie,
		newBucket:      db.ContractTrieContract,
		oldBuildPrefix: func(_ *felt.Address) []byte { return []byte{byte(db.StateTrie)} },
		newTrieID: func(_ *felt.Address) trieutils.TrieID {
			return trieutils.NewContractTrieID(felt.StateRootHash(felt.One))
		},
		hashFn:     crypto.Pedersen,
		buildOldFn: trie.NewTriePedersen,
	},
	{
		name:      "StorageTrie",
		oldBucket: db.ContractStorage,
		newBucket: db.ContractTrieStorage,
		owner:     felt.FromUint64[felt.Address](42),
		oldBuildPrefix: func(owner *felt.Address) []byte {
			ownerBytes := owner.Bytes()
			return db.ContractStorage.Key(ownerBytes[:])
		},
		newTrieID: func(owner *felt.Address) trieutils.TrieID {
			return trieutils.NewContractStorageTrieID(felt.StateRootHash(felt.One), *owner)
		},
		hashFn:     crypto.Pedersen,
		buildOldFn: trie.NewTriePedersen,
	},
}

var transcoderCases = []struct {
	name   string
	leaves leafMap
}{
	{"empty trie", nil},
	{"single leaf", leafMap{
		felt.FromUint64[felt.Felt](1): felt.FromUint64[felt.Felt](100),
	}},
	{"deep split", leafMap{
		felt.FromUint64[felt.Felt](2): felt.FromUint64[felt.Felt](10),
		felt.FromUint64[felt.Felt](3): felt.FromUint64[felt.Felt](20),
	}},
	{"left right split", leafMap{
		felt.FromUint64[felt.Felt](1):           felt.FromUint64[felt.Felt](10),
		felt.FromBytes[felt.Felt]([]byte{0x04}): felt.FromUint64[felt.Felt](20), // 2^250
	}},
	{"full depth 2 tree", leafMap{
		felt.FromUint64[felt.Felt](1):           felt.FromUint64[felt.Felt](10), // 00...
		felt.FromBytes[felt.Felt]([]byte{0x02}): felt.FromUint64[felt.Felt](20), // 01...
		felt.FromBytes[felt.Felt]([]byte{0x04}): felt.FromUint64[felt.Felt](30), // 10...
		felt.FromBytes[felt.Felt]([]byte{0x06}): felt.FromUint64[felt.Felt](40), // 11...
	}},
	{"hundred sequential leaves", func() leafMap {
		leaves := make(leafMap, 100)
		for i := 1; i <= 100; i++ {
			leaves[felt.FromUint64[felt.Felt](uint64(i))] = felt.FromUint64[felt.Felt](uint64(i) * 7)
		}
		return leaves
	}()},
	{"random 1000 leaves", randomLeaves(1000)},
}

func TestMigrate_FreshDBIsNoOp(t *testing.T) {
	memDB := memory.New()

	state, err := (&trielib.Migrator{}).Migrate(
		context.Background(),
		memDB,
		nil,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	assert.Nil(
		t,
		state,
		"fresh DB must mark migration applied (nil intermediate state) without doing work",
	)
}

func TestMigrate_RunsWhenOldDataPresent(t *testing.T) {
	leaves := randomLeaves(100)
	memDB := buildFullDB(t, leaves)

	require.True(t, bucketHasKeys(t, memDB, db.ClassesTrie), "precondition: DB has old-format data")

	state, err := (&trielib.Migrator{}).Migrate(
		context.Background(),
		memDB,
		nil,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	assert.Nil(t, state, "completed migration must return nil intermediate state")

	for _, bucket := range []db.Bucket{db.ClassesTrie, db.StateTrie, db.ContractStorage} {
		assert.False(t,
			bucketHasKeys(t, memDB, bucket),
			"old-format bucket %v should be empty after migration", bucket)
	}
}

// TestMigrationEndToEnd verifies that the migration produces byte-for-byte
// identical DB output to a natively-built trie2 for all three trie types and
// all leaf counts. Catches encoding bugs that root-hash comparison cannot.
func TestMigrationEndToEnd(t *testing.T) {
	type testCase struct {
		name   string
		tc     trieCase
		leaves leafMap
	}

	var cases []testCase
	for _, tc := range trieCases {
		for _, lc := range transcoderCases {
			cases = append(cases, testCase{
				name:   tc.name + "/" + lc.name,
				tc:     tc,
				leaves: lc.leaves,
			})
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prefix := c.tc.oldBuildPrefix(&c.tc.owner)

			migratedDB := memory.New()
			buildDeprecatedTrie(t, migratedDB, c.leaves, c.tc.buildOldFn, prefix)
			_, err := (&trielib.Migrator{}).Migrate(
				context.Background(), migratedDB, nil, log.NewNopZapLogger(),
			)
			require.NoError(t, err)

			nativeDB := memory.New()
			buildTrie(t, nativeDB, c.leaves,
				c.tc.newTrieID(&c.tc.owner), c.tc.hashFn, c.tc.newBucket)

			assert.Equal(t,
				allKeysUnder(t, nativeDB, c.tc.newBucket),
				allKeysUnder(t, migratedDB, c.tc.newBucket))
		})
	}
}

// TestMigrationIsResumable verifies that re-running migration over a DB
// whose class-trie destination root is already present skips the class
// migration and only finishes the contract trie. The "partial state" is
// faked by copying the reference DB's new-format class-trie keys into the
// partial DB before running migration.
func TestMigrationIsResumable(t *testing.T) {
	leaves := randomLeaves(1000)

	// Reference: full migration from scratch.
	refDB := buildFullDB(t, leaves)
	_, err := (&trielib.Migrator{}).Migrate(context.Background(), refDB, nil, log.NewNopZapLogger())
	require.NoError(t, err)

	// Partial DB: both tries in old format initially.
	partialDB := buildFullDB(t, leaves)

	// Fake a prior successful class-trie migration by copying refDB's
	// new-format class-trie keys directly into partialDB.
	refClassKeys := allKeysUnder(t, refDB, db.ClassTrie)
	require.NotEmpty(t, refClassKeys, "reference class trie should be non-empty")
	for k, v := range refClassKeys {
		require.NoError(t, partialDB.Put([]byte(k), v))
	}

	// Resume: migration should skip the class trie (its dest root is present)
	// and complete only the contract trie.
	_, err = (&trielib.Migrator{}).Migrate(context.Background(), partialDB, nil, log.NewNopZapLogger())
	require.NoError(t, err)

	// Final state must match the reference full-run output for every new-format bucket.
	for _, bucket := range []db.Bucket{db.ClassTrie, db.ContractTrieContract} {
		refKeys := allKeysUnder(t, refDB, bucket)
		resumedKeys := allKeysUnder(t, partialDB, bucket)
		assert.Equal(t, refKeys, resumedKeys,
			"resumed migration result differs from full run for bucket %v", bucket)
	}
}

// TestMigrationMultiStorageOwners exercises enumerateStorageTries across
// multiple owners (scanTrie's prefix-leave path) and keeps all 4 ingestor
// workers busy by giving them 7 tries to chew through (2 global + 5 storage).
func TestMigrationMultiStorageOwners(t *testing.T) {
	leaves := randomLeaves(50)

	migratedDB := memory.New()
	buildDeprecatedTrie(t, migratedDB, leaves, trie.NewTriePoseidon, db.ClassesTrie.Key())
	buildDeprecatedTrie(t, migratedDB, leaves, trie.NewTriePedersen, db.StateTrie.Key())

	owners := []felt.Address{
		felt.FromUint64[felt.Address](1),
		felt.FromUint64[felt.Address](2),
		felt.FromUint64[felt.Address](3),
		felt.FromUint64[felt.Address](42),
		felt.FromUint64[felt.Address](999),
	}
	for _, owner := range owners {
		ownerBytes := owner.Bytes()
		buildDeprecatedTrie(t, migratedDB, leaves, trie.NewTriePedersen,
			db.ContractStorage.Key(ownerBytes[:]))
	}

	_, err := (&trielib.Migrator{}).Migrate(
		context.Background(),
		migratedDB,
		nil,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)

	// Per-owner native build → assert every native key is present (with the
	migratedAll := allKeysUnder(t, migratedDB, db.ContractTrieStorage)
	for _, owner := range owners {
		nativeDB := memory.New()
		id := trieutils.NewContractStorageTrieID(felt.StateRootHash(felt.One), owner)
		buildTrie(t, nativeDB, leaves, id, crypto.Pedersen, db.ContractTrieStorage)
		for k, v := range allKeysUnder(t, nativeDB, db.ContractTrieStorage) {
			gotV, ok := migratedAll[k]
			require.True(t, ok, "owner %v missing key", owner)
			assert.Equal(t, v, gotV, "owner %v value differs at key", owner)
		}
	}

	// Old buckets fully drained.
	for _, bucket := range []db.Bucket{db.ClassesTrie, db.StateTrie, db.ContractStorage} {
		assert.False(t, bucketHasKeys(t, migratedDB, bucket),
			"old bucket %v should be drained", bucket)
	}
}

// TestMigrationIdempotent verifies that a successful migration is a no-op on
// a second run: needsMigration sees the wiped deprecated buckets and returns
// early without touching the migrated state.
func TestMigrationIsNoopOnSecondRun(t *testing.T) {
	leaves := randomLeaves(100)
	memDB := buildFullDB(t, leaves)

	state, err := (&trielib.Migrator{}).Migrate(
		context.Background(),
		memDB,
		nil,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, state)
	snapshot := snapshotAllBuckets(t, memDB,
		db.ClassTrie, db.ContractTrieContract, db.ContractTrieStorage)

	state, err = (&trielib.Migrator{}).Migrate(context.Background(), memDB, nil, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)
	require.Equal(t, snapshot,
		snapshotAllBuckets(t, memDB,
			db.ClassTrie, db.ContractTrieContract, db.ContractTrieStorage),
		"second Migrate call must not change state")
}

// TestMigrationCancelledContext verifies that a pre-cancelled ctx surfaces
// context.Canceled with the shouldRerun sentinel, and that a fresh ctx
// completes the migration normally afterwards.
func TestMigrationCancelledContext(t *testing.T) {
	leaves := randomLeaves(100)
	memDB := buildFullDB(t, leaves)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state, err := (&trielib.Migrator{}).Migrate(ctx, memDB, nil, log.NewNopZapLogger())
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, state, "shouldRerun sentinel must not be nil")
	require.Empty(t, state, "shouldRerun is a non-nil empty slice")

	state, err = (&trielib.Migrator{}).Migrate(context.Background(), memDB, nil, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)
}

// buildFullDB creates an old-format DB populated with a class, a contract, and
// one storage trie, all built from the same leaf set.
func buildFullDB(t *testing.T, leaves leafMap) db.KeyValueStore {
	t.Helper()
	memDB := memory.New()

	owner := felt.FromUint64[felt.Address](42)
	ownerBytes := owner.Bytes()
	storagePrefix := db.ContractStorage.Key(ownerBytes[:])

	buildDeprecatedTrie(t, memDB, leaves, trie.NewTriePoseidon, db.ClassesTrie.Key())
	buildDeprecatedTrie(t, memDB, leaves, trie.NewTriePedersen, db.StateTrie.Key())
	buildDeprecatedTrie(t, memDB, leaves, trie.NewTriePedersen, storagePrefix)

	return memDB
}

func buildDeprecatedTrie(
	t *testing.T,
	database db.KeyValueStore,
	leaves leafMap,
	//nolint:staticcheck // Necessary for old state
	trieFn func(db.IndexedBatch, []byte, uint8) (*trie.Trie, error),
	prefix []byte,
) felt.Felt {
	t.Helper()
	//nolint:staticcheck // Necessary for old state
	txn := database.NewIndexedBatch()
	tr, err := trieFn(txn, prefix, 251)
	require.NoError(t, err)
	for key, value := range leaves {
		_, err := tr.Put(&key, &value)
		require.NoError(t, err)
	}
	root, err := tr.Root()
	require.NoError(t, err)
	require.NoError(t, tr.Commit())
	require.NoError(t, txn.Write())
	return root
}

func buildTrie(
	t *testing.T,
	kvStore db.KeyValueStore,
	leaves leafMap,
	id trieutils.TrieID,
	hashFn crypto.HashFn,
	newBucket db.Bucket,
) {
	t.Helper()
	rawDB := rawdb.New(kvStore)
	tr, err := trie2.New(id, 251, hashFn, rawDB)
	require.NoError(t, err)
	for key, value := range leaves {
		require.NoError(t, tr.Update(&key, &value))
	}
	root, nodes := tr.Commit()
	if nodes == nil {
		return // empty trie — nothing to persist
	}
	mergeSet := trienode.NewMergeNodeSet(nodes)
	var zero felt.StateRootHash
	stateRoot := felt.StateRootHash(root)
	batch := kvStore.NewBatch()
	if newBucket == db.ClassTrie {
		require.NoError(t, rawDB.Update(&stateRoot, &zero, 0, mergeSet, nil, batch))
	} else {
		require.NoError(t, rawDB.Update(&stateRoot, &zero, 0, nil, mergeSet, batch))
	}
	require.NoError(t, batch.Write())
}

func randomLeaves(n int) leafMap {
	leaves := make(leafMap, n)
	for len(leaves) < n {
		var k, v felt.Felt
		k.SetRandom()
		v.SetRandom()
		leaves[k] = v
	}
	return leaves
}

func snapshotAllBuckets(t *testing.T, r db.KeyValueReader, buckets ...db.Bucket) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte)
	for _, b := range buckets {
		for k, v := range allKeysUnder(t, r, b) {
			out[k] = v
		}
	}
	return out
}

func allKeysUnder(t *testing.T, r db.KeyValueReader, bucket db.Bucket) map[string][]byte {
	t.Helper()
	prefix := bucket.Key()
	iter, err := r.NewIterator(prefix, true)
	require.NoError(t, err)
	defer iter.Close()
	out := make(map[string][]byte)
	for ok := iter.First(); ok; ok = iter.Next() {
		val, err := iter.Value()
		require.NoError(t, err)
		out[string(iter.Key())] = val
	}
	return out
}

func bucketHasKeys(t *testing.T, r db.KeyValueReader, bucket db.Bucket) bool {
	t.Helper()
	it, err := r.NewIterator(bucket.Key(), true)
	require.NoError(t, err)
	defer it.Close()
	return it.First()
}
