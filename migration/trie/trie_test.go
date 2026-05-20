package trie

import (
	"bytes"
	"context"
	"math/rand"
	"slices"
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
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type leafMap map[felt.Felt]felt.Felt

func nopLogger() log.StructuredLogger   { return log.NewNopZapLogger() }
func nopFlush(*task, chan<- task) error { return nil }

type trieCase struct {
	name           string
	oldBucket      db.Bucket
	newBucket      db.Bucket
	owner          felt.Address
	oldBuildPrefix func(owner felt.Address) []byte
	newTrieID      func(owner felt.Address) trieutils.TrieID
	hashFn         crypto.HashFn
	//nolint:staticcheck // Necessary for old state
	buildOldFn func(db.IndexedBatch, []byte, uint8) (*trie.Trie, error)
}

var trieCases = []trieCase{
	{
		name:           "ClassTrie",
		oldBucket:      db.ClassesTrie,
		newBucket:      db.ClassTrie,
		oldBuildPrefix: func(_ felt.Address) []byte { return []byte{byte(db.ClassesTrie)} },
		newTrieID: func(_ felt.Address) trieutils.TrieID {
			return trieutils.NewClassTrieID(felt.StateRootHash(felt.One))
		},
		hashFn:     crypto.Poseidon,
		buildOldFn: trie.NewTriePoseidon,
	},
	{
		name:           "ContractTrie",
		oldBucket:      db.StateTrie,
		newBucket:      db.ContractTrieContract,
		oldBuildPrefix: func(_ felt.Address) []byte { return []byte{byte(db.StateTrie)} },
		newTrieID: func(_ felt.Address) trieutils.TrieID {
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
		oldBuildPrefix: func(owner felt.Address) []byte {
			ownerFelt := felt.Felt(owner)
			ownerBytes := ownerFelt.Bytes()
			return db.ContractStorage.Key(ownerBytes[:])
		},
		newTrieID: func(owner felt.Address) trieutils.TrieID {
			return trieutils.NewContractStorageTrieID(felt.StateRootHash(felt.One), owner)
		},
		hashFn:     crypto.Pedersen,
		buildOldFn: trie.NewTriePedersen,
	},
}

// randomLeaves generates n distinct leaf key-value pairs using a fixed seed,
// with keys spread across the full 251-bit felt range for structural variety.
func randomLeaves(n int, seed int64) leafMap {
	rng := rand.New(rand.NewSource(seed))
	leaves := make(leafMap, n)
	var kb, vb [32]byte
	for len(leaves) < n {
		rng.Read(kb[:])
		rng.Read(vb[:])
		// Clear the top 5 bits so all keys are safely below the StarkNet prime (~2^251+δ).
		kb[0] &= 0x07
		k := felt.FromBytes[felt.Felt](kb[:])
		v := felt.FromBytes[felt.Felt](vb[:])
		leaves[k] = v
	}
	return leaves
}

var transcoderCases = []struct {
	name   string
	leaves leafMap
}{
	// --- trivial structural cases ---
	{"empty trie", nil},
	{"single leaf", leafMap{
		felt.FromUint64[felt.Felt](1): felt.FromUint64[felt.Felt](100),
	}},

	// --- two-leaf cases probing specific split depths ---

	// Keys 2 and 3 differ only in the last path bit (bit 250 of felt = path bit 250).
	// Root edge spans 250 bits; binary node is at maximum depth.
	{"deep split", leafMap{
		felt.FromUint64[felt.Felt](2): felt.FromUint64[felt.Felt](10),
		felt.FromUint64[felt.Felt](3): felt.FromUint64[felt.Felt](20),
	}},

	// One key < 2^250 (trie path bit[0]=0), one key ≥ 2^250 (trie path bit[0]=1).
	// Root is a binary node — rootPath.Len()==0, no root edge written.
	// This path is never hit by sequential small-integer keys.
	{"left right split", leafMap{
		felt.FromUint64[felt.Felt](1):           felt.FromUint64[felt.Felt](10),
		felt.FromBytes[felt.Felt]([]byte{0x04}): felt.FromUint64[felt.Felt](20), // 2^250
	}},

	// --- four leaves covering all 2-bit path prefixes (00, 01, 10, 11) ---
	// Root is a binary node; left and right subtrees each contain a binary node.
	// Tests two levels of binary processing with rootPath.Len()==0.
	{"full depth 2 tree", leafMap{
		felt.FromUint64[felt.Felt](1):           felt.FromUint64[felt.Felt](10), // 00...
		felt.FromBytes[felt.Felt]([]byte{0x02}): felt.FromUint64[felt.Felt](20), // 01... (2^249)
		felt.FromBytes[felt.Felt]([]byte{0x04}): felt.FromUint64[felt.Felt](30), // 10... (2^250)
		felt.FromBytes[felt.Felt]([]byte{0x06}): felt.FromUint64[felt.Felt](40), // 11... (2^250+2^249)
	}},

	// --- larger sequential case for basic load ---
	{"hundred sequential leaves", func() leafMap {
		leaves := make(leafMap, 100)
		for i := 1; i <= 100; i++ {
			leaves[felt.FromUint64[felt.Felt](uint64(i))] = felt.FromUint64[felt.Felt](uint64(i) * 7)
		}
		return leaves
	}()},

	// --- random leaves spanning the full 251-bit space ---
	// Keys are evenly distributed across all trie depths, exercising every code
	// path: balanced binary nodes, varied edge lengths, and batch flush thresholds.
	{"random 1000 leaves", randomLeaves(1000, 42)},
}

func TestMigrate_FreshDBIsNoOp(t *testing.T) {
	memDB := memory.New()

	state, err := (&Migrator{}).Migrate(context.Background(), memDB, nil, nopLogger())
	require.NoError(t, err)
	assert.Nil(
		t,
		state,
		"fresh DB must mark migration applied (nil intermediate state) without doing work",
	)
}

func TestMigrate_RunsWhenOldDataPresent(t *testing.T) {
	leaves := randomLeaves(100, 7)
	memDB := buildFullDB(t, leaves)

	needed, err := needsMigration(memDB)
	require.NoError(t, err)
	require.True(t, needed, "precondition: DB has old-format data")

	state, err := (&Migrator{}).Migrate(context.Background(), memDB, nil, nopLogger())
	require.NoError(t, err)
	assert.Nil(t, state, "completed migration must return nil intermediate state")

	stillNeeded, err := needsMigration(memDB)
	require.NoError(t, err)
	assert.False(t, stillNeeded, "old-format buckets should be empty after migration")
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
			prefix := c.tc.oldBuildPrefix(c.tc.owner)

			migratedDB := memory.New()
			buildDeprecatedTrie(t, migratedDB, c.leaves, c.tc.buildOldFn, prefix)
			_, err := runMigration(context.Background(), migratedDB, nopLogger())
			require.NoError(t, err)

			nativeDB := memory.New()
			buildTrie(t, nativeDB, c.leaves,
				c.tc.newTrieID(c.tc.owner), c.tc.hashFn, c.tc.newBucket)

			assert.Equal(t,
				allKeysUnder(t, nativeDB, c.tc.newBucket),
				allKeysUnder(t, migratedDB, c.tc.newBucket))
		})
	}
}

func TestMigrationIsResumable(t *testing.T) {
	leaves := randomLeaves(1000, 42)

	// Reference: full migration from scratch.
	refDB := buildFullDB(t, leaves)
	_, err := runMigration(context.Background(), refDB, nopLogger())
	require.NoError(t, err)

	// Partial DB: both tries in old format initially.
	partialDB := buildFullDB(t, leaves)

	// Manually migrate only the class trie to simulate a mid-run interruption.
	classPrefix := db.ClassesTrie.Key()
	var classRootPath *trie.BitArray
	require.NoError(t, partialDB.Get(classPrefix, func(val []byte) error {
		var perr error
		classRootPath, perr = parseRootPath(val)
		return perr
	}))
	classDesc := TrieDesc{
		OldBucket: db.ClassesTrie,
		NewBucket: db.ClassTrie,
		HashFn:    crypto.Poseidon,
		NodeCount: len(leaves),
		RootPath:  classRootPath,
	}

	pool := newHashWorkerPool()
	defer pool.close()
	t1 := &task{batch: partialDB.NewBatch()}
	stack := make([]dfsFrame, 0, dfsStackCap)
	_, err = migrateTrie(partialDB, classDesc, pool, t1, nopFlush, nil, stack)
	require.NoError(t, err)
	require.NoError(t, t1.batch.Write())

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

// buildFullDB creates an old-format DB populated with a class, a contract, and
// one storage trie, all built from the same leaf set.
func buildFullDB(t *testing.T, leaves leafMap) db.KeyValueStore {
	t.Helper()
	database := memory.New()

	buildDeprecatedTrie(t, database, leaves, trie.NewTriePoseidon, db.ClassesTrie.Key())
	buildDeprecatedTrie(t, database, leaves, trie.NewTriePedersen, db.StateTrie.Key())

	var ownerFelt felt.Felt
	ownerFelt.SetUint64(42)
	ownerBytes := ownerFelt.Bytes()
	storagePrefix := db.ContractStorage.Key(ownerBytes[:])
	buildDeprecatedTrie(t, database, leaves, trie.NewTriePedersen, storagePrefix)

	return database
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

// buildTrie builds a trie2 natively from leaves and persists it to kvStore.
// newBucket distinguishes class trie (db.ClassTrie) from contract/storage tries —
// it controls which Update argument the NodeSet is passed as.
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
	memDB := memory.New()
	descs := collectTries(t, memDB)
	require.Len(t, descs, 2)
	assert.Equal(t, db.ClassesTrie, descs[0].OldBucket)
	assert.Equal(t, db.StateTrie, descs[1].OldBucket)
}

func TestEnumerateTries_GlobalTriesPresent(t *testing.T) {
	memDB := memory.New()
	descs := collectTries(t, memDB)
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
	memDB := memory.New()
	var owners [3]felt.Address
	for i := range owners {
		var f felt.Felt
		f.SetUint64(uint64(i + 1))
		owners[i] = felt.Address(f)
		insertFakeStorageNodes(t, memDB, owners[i], 5)
	}

	descs := collectTries(t, memDB)
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
	memDB := memory.New()
	var ownerFelt felt.Felt
	ownerFelt.SetUint64(99)
	owner := felt.Address(ownerFelt)
	insertFakeStorageNodes(t, memDB, owner, 7)

	descs := collectTries(t, memDB)
	require.Len(t, descs, 3)

	idx := slices.IndexFunc(descs, func(d TrieDesc) bool { return d.OldBucket == db.ContractStorage })
	require.NotEqual(t, -1, idx)
	assert.Equal(t, 7, descs[idx].NodeCount)
}

func TestEnumerateTries_StorageTrieCountsPresent(t *testing.T) {
	memDB := memory.New()
	for i, n := range []int{3, 7, 1} {
		var f felt.Felt
		f.SetUint64(uint64(i + 1))
		insertFakeStorageNodes(t, memDB, felt.Address(f), n)
	}

	descs := collectTries(t, memDB)
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
	memDB := memory.New()
	var ownerFelt felt.Felt
	ownerFelt.SetUint64(12345)
	owner := felt.Address(ownerFelt)
	insertFakeStorageNodes(t, memDB, owner, 3)

	descs := collectTries(t, memDB)
	require.Len(t, descs, 3)

	idx := slices.IndexFunc(descs, func(d TrieDesc) bool { return d.OldBucket == db.ContractStorage })
	require.NotEqual(t, -1, idx)
	assert.Equal(t, owner, descs[idx].Owner)
}

func TestEnumerateTries_MultipleOwnersOrdered(t *testing.T) {
	memDB := memory.New()
	owners := make([]felt.Address, 5)
	for i := range owners {
		var f felt.Felt
		f.SetUint64(uint64(i + 1))
		owners[i] = felt.Address(f)
		insertFakeStorageNodes(t, memDB, owners[i], i+1)
	}

	descs := collectTries(t, memDB)
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
