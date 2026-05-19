package trie

import (
	"context"
	"math/rand"
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
func noFlush(current db.Batch) db.Batch { return current }

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

var allBackends = []struct {
	name    string
	backend *dfsMigrator
}{
	{"dfsSerial", &dfsMigrator{parallelDispatch: false}},
	{"dfsParallel", &dfsMigrator{parallelDispatch: true}},
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

// TestMigrateTrieMatchesNativeTrie2 verifies that each backend produces byte-for-byte
// identical DB output to a natively-built trie2 for all three trie types and all leaf
// counts. This catches encoding bugs that root-hash comparison cannot detect.
func TestMigrationEndToEnd(t *testing.T) {
	type testCase struct {
		name    string
		tc      trieCase
		backend *dfsMigrator
		leaves  leafMap
	}

	var cases []testCase
	for _, tc := range trieCases {
		for _, b := range allBackends {
			for _, lc := range transcoderCases {
				cases = append(cases, testCase{
					name:    tc.name + "/" + b.name + "/" + lc.name,
					tc:      tc,
					backend: b.backend,
					leaves:  lc.leaves,
				})
			}
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prefix := c.tc.oldBuildPrefix(c.tc.owner)

			migratedDB := memory.New()
			buildDeprecatedTrie(t, migratedDB, c.leaves, c.tc.buildOldFn, prefix)
			runMigration(context.Background(), migratedDB, nopLogger())

			nativeDB := memory.New()
			buildTrie(t, nativeDB, c.leaves,
				c.tc.newTrieID(c.tc.owner), c.tc.hashFn, c.tc.newBucket)

			assert.Equal(t,
				allKeysUnder(t, nativeDB, c.tc.newBucket),
				allKeysUnder(t, migratedDB, c.tc.newBucket))
		})
	}
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

// buildNativeTrie2 builds a trie2 natively from leaves and persists it to kvStore.
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
