package trie_test

import (
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

type testKey struct {
	name     string
	key      *felt.Felt
	expected *felt.Felt
}

type testTrie struct {
	name     string
	buildFn  func(*testing.T) *trie.Trie
	height   uint8
	testKeys []testKey
}

// TODO(weiihann): tidy this up
func TestProve(t *testing.T) {
	t.Parallel()

	tests := []testTrie{
		{
			name:    "simple binary",
			buildFn: buildSimpleTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(3),
				},
			},
		},
		{
			name:    "simple double binary",
			buildFn: buildSimpleDoubleBinaryTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(2),
				},
				{
					name:     "prove existing key 3",
					key:      new(felt.Felt).SetUint64(3),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove non-existent key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existent key 123",
					key:      new(felt.Felt).SetUint64(123),
					expected: new(felt.Felt).SetUint64(0),
				},
			},
		},
		{
			name:    "simple binary root",
			buildFn: buildSimpleBinaryRootTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      new(felt.Felt).SetUint64(0),
					expected: utils.HexToFelt(t, "0xcc"),
				},
			},
		},
		{
			name: "left-right edge",
			buildFn: func(t *testing.T) *trie.Trie {
				memdb := pebble.NewMemTest(t)
				txn, err := memdb.NewTransaction(true)
				require.NoError(t, err)

				tr, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{1}), 251)
				require.NoError(t, err)

				key := utils.HexToFelt(t, "0xff")
				value := utils.HexToFelt(t, "0xaa")

				_, err = tr.Put(key, value)
				require.NoError(t, err)
				require.NoError(t, tr.Commit())
				return tr
			},
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      utils.HexToFelt(t, "0xff"),
					expected: utils.HexToFelt(t, "0xaa"),
				},
			},
		},
		{
			name:    "three key trie",
			buildFn: build3KeyTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(6),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr := test.buildFn(t)

			for _, tc := range test.testKeys {
				t.Run(tc.name, func(t *testing.T) {
					proofSet := trie.NewProofNodeSet()
					err := tr.Prove(tc.key, proofSet)
					require.NoError(t, err)

					root, err := tr.Root()
					require.NoError(t, err)

					key := tr.FeltToKey(tc.key)
					val, err := trie.VerifyProof(root, &key, proofSet, crypto.Pedersen)
					require.NoError(t, err)
					require.Equal(t, tc.expected, val)
				})
			}
		})
	}
}

func TestProveNKeys(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie := buildTrieWithNKeys(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i))
		key := tempTrie.FeltToKey(keyFelt)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keyFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &key, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", key.String())
		}
		require.Equal(t, val, keyFelt)
	}
}

func TestProveNKeysWithNonExistentKeys(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie := buildTrieWithNKeys(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i + n))
		key := tempTrie.FeltToKey(keyFelt)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keyFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &key, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", key.String())
		}
		require.Equal(t, &felt.Zero, val)
	}
}

func TestProveRandomTrie(t *testing.T) {
	n := 1000
	tempTrie, keys := buildRandomTrie(t, n)

	for i := 0; i < n; i++ {
		key := tempTrie.FeltToKey(keys[i])

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keys[i], proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &key, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", keys[i].String())
		}
		require.Equal(t, val, keys[i])
	}
}

func TestProofToPath(t *testing.T) {
	testScenarios := []testTrie{
		{
			name:    "one key trie",
			buildFn: build1KeyTrie,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove non-existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(1),
				},
				{
					name:     "prove non-existing key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 0xff...ff",
					key:      utils.HexToFelt(t, "0xffffffffffffffffffffffffffffffff"),
					expected: new(felt.Felt).SetUint64(0),
				},
			},
		},
		{
			name:    "simple trie",
			buildFn: buildSimpleTrie,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(2),
				},
				{
					name:     "prove existing key 2",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(3),
				},
			},
		},
		{
			name:    "simple binary root trie",
			buildFn: buildSimpleBinaryRootTrie,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: utils.HexToFelt(t, "0xcc"),
				},
				{
					name:     "prove existing key 0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					key:      utils.HexToFelt(t, "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
					expected: utils.HexToFelt(t, "0xdd"),
				},
			},
		},
		{
			name:    "simple double binary trie",
			buildFn: buildSimpleDoubleBinaryTrie,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(2),
				},
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(3),
				},
				{
					name:     "prove existing key 3",
					key:      new(felt.Felt).SetUint64(3),
					expected: new(felt.Felt).SetUint64(5),
				},
			},
		},
		{
			name:    "Starknet docs example trie",
			buildFn: buildStarknetDocsTrie,
			height:  3,
			testKeys: []testKey{
				{
					name:     "prove non-existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(1),
				},
				{
					name:     "prove non-existing key 3",
					key:      new(felt.Felt).SetUint64(3),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 4",
					key:      new(felt.Felt).SetUint64(4),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 5",
					key:      new(felt.Felt).SetUint64(5),
					expected: new(felt.Felt).SetUint64(1),
				},
				{
					name:     "prove non-existing key 6",
					key:      new(felt.Felt).SetUint64(6),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 7",
					key:      new(felt.Felt).SetUint64(7),
					expected: new(felt.Felt).SetUint64(0),
				},
			},
		},
		{
			name:    "3 keys trie",
			buildFn: build3KeyTrie,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(4),
				},
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove existing key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(6),
				},
			},
		},
		{
			name:    "4 keys trie A",
			buildFn: build4KeysTrieA,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(4),
				},
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(6),
				},
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(4),
					expected: new(felt.Felt).SetUint64(7),
				},
			},
		},
		{
			name:    "4 keys trie B",
			buildFn: build4KeysTrieB,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove non-existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(4),
				},
				{
					name:     "prove existing key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove existing key 3",
					key:      new(felt.Felt).SetUint64(3),
					expected: new(felt.Felt).SetUint64(6),
				},
				{
					name:     "prove existing key 4",
					key:      new(felt.Felt).SetUint64(4),
					expected: new(felt.Felt).SetUint64(7),
				},
				{
					name:     "prove non-existing key 5",
					key:      new(felt.Felt).SetUint64(5),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 6",
					key:      new(felt.Felt).SetUint64(6),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 7",
					key:      new(felt.Felt).SetUint64(7),
					expected: new(felt.Felt).SetUint64(0),
				},
			},
		},
		{
			name:    "4 keys trie C",
			buildFn: build4KeysTrieC,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove non-existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(4),
				},
				{
					name:     "prove non-existing key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 3",
					key:      new(felt.Felt).SetUint64(4),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove non-existing key 4",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 5",
					key:      new(felt.Felt).SetUint64(5),
					expected: new(felt.Felt).SetUint64(6),
				},
				{
					name:     "prove non-existing key 6",
					key:      new(felt.Felt).SetUint64(6),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 7",
					key:      new(felt.Felt).SetUint64(7),
					expected: new(felt.Felt).SetUint64(7),
				},
			},
		},
		{
			name:    "4 keys trie D",
			buildFn: build4KeysTrieD,
			height:  251,
			testKeys: []testKey{
				{
					name:     "prove non-existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 1",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(4),
				},
				{
					name:     "prove non-existing key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existing key 3",
					key:      new(felt.Felt).SetUint64(3),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 4",
					key:      new(felt.Felt).SetUint64(4),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove non-existing key 5",
					key:      new(felt.Felt).SetUint64(5),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove existing key 6",
					key:      new(felt.Felt).SetUint64(6),
					expected: new(felt.Felt).SetUint64(6),
				},
				{
					name:     "prove existing key 7",
					key:      new(felt.Felt).SetUint64(7),
					expected: new(felt.Felt).SetUint64(7),
				},
			},
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			tr := scenario.buildFn(t)

			for _, tc := range scenario.testKeys {
				t.Run(tc.name, func(t *testing.T) {
					proofSet := trie.NewProofNodeSet()
					err := tr.Prove(tc.key, proofSet)
					require.NoError(t, err)

					nodes := trie.NewStorageNodeSet()
					root, err := tr.Root()
					require.NoError(t, err)
					keyFelt := tr.FeltToKey(tc.key)
					err = trie.ProofToPath(root, &keyFelt, proofSet, nodes)
					require.NoError(t, err)

					tr2, err := trie.BuildTrie(scenario.height, nodes.List())
					require.NoError(t, err)

					root2, err := tr2.Root()
					require.NoError(t, err)
					require.Equal(t, root, root2)

					// Verify value
					value, err := tr2.Get(tc.key)
					require.NoError(t, err)
					require.Equal(t, tc.expected, value)
				})
			}
		})
	}
}

func TestProofToPathNKeys(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie := buildTrieWithNKeys(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i))
		key := tempTrie.FeltToKey(keyFelt)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keyFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		nodes := trie.NewStorageNodeSet()
		err = trie.ProofToPath(root, &key, proofSet, nodes)
		require.NoError(t, err)

		tr2, err := trie.BuildTrie(251, nodes.List())
		require.NoError(t, err)

		root2, err := tr2.Root()
		require.NoError(t, err)
		require.Equal(t, root, root2)

		// Verify value
		value, err := tr2.Get(keyFelt)
		require.NoError(t, err)
		require.Equal(t, value, keyFelt)
	}
}

func TestProofToPathNKeysWithNonExistentKeys(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie := buildTrieWithNKeys(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i + n))
		key := tempTrie.FeltToKey(keyFelt)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keyFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		nodes := trie.NewStorageNodeSet()
		err = trie.ProofToPath(root, &key, proofSet, nodes)
		require.NoError(t, err)

		tr2, err := trie.BuildTrie(251, nodes.List())
		require.NoError(t, err)

		root2, err := tr2.Root()
		if err != nil {
			t.Fatalf("failed for i %d, err: %s", i, err)
		}
		require.Equal(t, root, root2)
	}
}

func TestProofToPathRandomTrie(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie, keys := buildRandomTrie(t, n)

	for i := 0; i < n; i++ {
		key := tempTrie.FeltToKey(keys[i])

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keys[i], proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		nodes := trie.NewStorageNodeSet()
		err = trie.ProofToPath(root, &key, proofSet, nodes)
		require.NoError(t, err)

		tr2, err := trie.BuildTrie(251, nodes.List())
		require.NoError(t, err)

		root2, err := tr2.Root()
		require.NoError(t, err)
		require.Equal(t, root, root2)
	}
}

func buildTrieWithKeysAndValues(t *testing.T, height uint8, keys, values []*felt.Felt) *trie.Trie {
	if len(keys) != len(values) {
		t.Fatal("keys and values must have same length")
	}

	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), height)
	require.NoError(t, err)

	for i := 0; i < len(keys); i++ {
		_, err = tempTrie.Put(keys[i], values[i])
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	return tempTrie
}

func build1KeyTrie(t *testing.T) *trie.Trie {
	return buildTrieWithNKeys(t, 1)
}

func buildSimpleTrie(t *testing.T) *trie.Trie {
	//   (250, 0, x1)		edge
	//        |
	//     (0,0,x1)			binary
	//      /    \
	//     (2)  (3)
	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(2),
		new(felt.Felt).SetUint64(3),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func buildSimpleBinaryRootTrie(t *testing.T) *trie.Trie {
	// PF
	//           (0, 0, x)
	//    /                    \
	// (250, 0, cc)     (250, 11111.., dd)
	//    |                     |
	//   (cc)                  (dd)

	//	JUNO
	//           (0, 0, x)
	//    /                    \
	// (251, 0, cc)     (251, 11111.., dd)

	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		utils.HexToFelt(t, "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	}
	values := []*felt.Felt{
		utils.HexToFelt(t, "0xcc"),
		utils.HexToFelt(t, "0xdd"),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func buildSimpleDoubleBinaryTrie(t *testing.T) *trie.Trie {
	//           (249,0,x3)         // Edge
	//               |
	//           (0, 0, x3)         // Binary
	//         /            \
	//     (0,0,x1) // B  (1, 1, 5) // Edge leaf
	//      /    \             |
	//     (2)  (3)           (5)

	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(3),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(2),
		new(felt.Felt).SetUint64(3),
		new(felt.Felt).SetUint64(5),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func build3KeyTrie(t *testing.T) *trie.Trie {
	// 			Starknet
	//			--------
	//
	//			Edge
	//			|
	//			Binary with len 249				 parent
	//		 /				\
	//	Binary (250)	Edge with len 250
	//	/	\				/
	// 0x4	0x5			0x6						 child

	//			 Juno
	//			 ----
	//
	//		Node (path 249)
	//		/			\
	//  Node (binary)	 \
	//	/	\			 /
	// 0x4	0x5		   0x6

	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(2),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(6),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func buildStarknetDocsTrie(t *testing.T) *trie.Trie {
	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(2),
		new(felt.Felt).SetUint64(3),
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(6),
		new(felt.Felt).SetUint64(7),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(0),
	}
	return buildTrieWithKeysAndValues(t, 3, keys, values)
}

func build4KeysTrieA(t *testing.T) *trie.Trie {
	//			Juno
	//			248
	// 			/  \
	// 		249		\
	//		/ \		 \
	//	 250   \	  \
	//   / \   /\	  /\
	//  0   1 2	     4

	//			Juno - should be able to reconstruct this from proofs
	//			248
	// 			/  \
	// 		249			// Note we cant derive the right key, but need to store it's hash
	//		/ \
	//	 250   \
	//   / \   / (Left hash set, no key)
	//  0

	//			Pathfinder (???)
	//			  0	 Edge
	// 			  |
	// 			 248	Binary
	//			 / \
	//		   249	\		Binary Edge		??
	//	   	   / \	 \
	//		250  250  250		Binary Edge		??
	//	    / \   /    /
	// 	   0   1  2   4

	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(0),
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(2),
		new(felt.Felt).SetUint64(4),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(6),
		new(felt.Felt).SetUint64(7),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func build4KeysTrieB(t *testing.T) *trie.Trie {
	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(2),
		new(felt.Felt).SetUint64(3),
		new(felt.Felt).SetUint64(4),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(6),
		new(felt.Felt).SetUint64(7),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func build4KeysTrieC(t *testing.T) *trie.Trie {
	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(7),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(6),
		new(felt.Felt).SetUint64(7),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func build4KeysTrieD(t *testing.T) *trie.Trie {
	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(6),
		new(felt.Felt).SetUint64(7),
	}
	values := []*felt.Felt{
		new(felt.Felt).SetUint64(4),
		new(felt.Felt).SetUint64(5),
		new(felt.Felt).SetUint64(6),
		new(felt.Felt).SetUint64(7),
	}
	return buildTrieWithKeysAndValues(t, 251, keys, values)
}

func buildTrieWithNKeys(t *testing.T, numKeys int) *trie.Trie {
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)

	for i := 1; i < numKeys+1; i++ {
		key := new(felt.Felt).SetUint64(uint64(i))
		_, err := tempTrie.Put(key, key)
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	return tempTrie
}

func buildRandomTrie(t *testing.T, n int) (*trie.Trie, []*felt.Felt) {
	rrand := rand.New(rand.NewSource(3))

	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), uint8(rrand.Uint32()))
	require.NoError(t, err)

	keys := make([]*felt.Felt, n)
	for i := 0; i < n; i++ {
		keys[i] = new(felt.Felt).SetUint64(rrand.Uint64() + 1)
		_, err := tempTrie.Put(keys[i], keys[i])
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	return tempTrie, keys
}
