package trie_test

import (
	"math/rand"
	"sort"
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
	buildFn  func(*testing.T) (*trie.Trie, []*keyValue)
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
			buildFn: func(t *testing.T) (*trie.Trie, []*keyValue) {
				memdb := pebble.NewMemTest(t)
				txn, err := memdb.NewTransaction(true)
				require.NoError(t, err)

				tr, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{1}), 251)
				require.NoError(t, err)

				records := []*keyValue{
					{key: utils.HexToFelt(t, "0xff"), value: utils.HexToFelt(t, "0xaa")},
				}

				for _, record := range records {
					_, err = tr.Put(record.key, record.value)
					require.NoError(t, err)
				}
				require.NoError(t, tr.Commit())
				return tr, records
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
			tr, _ := test.buildFn(t)

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
	tempTrie, records := nonRandomTrie(t, n)

	for _, record := range records {
		key := tempTrie.FeltToKey(record.key)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &key, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", key.String())
		}
		require.Equal(t, record.value, val)
	}
}

func TestProveNKeyshNonExistent(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie, _ := nonRandomTrie(t, n)

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
	t.Parallel()
	tempTrie, records := randomTrie(t, 1000)

	for _, record := range records {
		key := tempTrie.FeltToKey(record.key)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &key, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", record.key.String())
		}
		require.Equal(t, record.value, val)
	}
}

// TestRangeProof tests normal range proof with both edge proofs
func TestRangeProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Root()
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		start := rand.Intn(n)
		end := rand.Intn(n-start) + start + 1

		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[start].key, records[end-1].key, proof)
		require.NoError(t, err)

		keys := []*felt.Felt{}
		values := []*felt.Felt{}
		for i := start; i < end; i++ {
			keys = append(keys, records[i].key)
			values = append(values, records[i].value)
		}

		_, err = trie.VerifyRangeProof(root, records[start].key, keys, values, proof)
		require.NoError(t, err)
	}
}

// TestRangeProofWithNonExistentProof tests normal range proof with non-existent proofs
func TestRangeProofWithNonExistentProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Root()
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		start := rand.Intn(n)
		end := rand.Intn(n-start) + start + 1

		first := decrementFelt(records[start].key)
		if start != 0 && first.Equal(records[start-1].key) {
			continue
		}

		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(first, records[end-1].key, proof)
		require.NoError(t, err)

		keys := make([]*felt.Felt, end-start)
		values := make([]*felt.Felt, end-start)
		for i := start; i < end; i++ {
			keys[i-start] = records[i].key
			values[i-start] = records[i].value
		}

		_, err = trie.VerifyRangeProof(root, first, keys, values, proof)
		require.NoError(t, err)
	}
}

// TestRangeProofWithInvalidNonExistentProof tests range proof with invalid non-existent proofs.
// One scenario is when there is a gap between the first element and the left edge proof.
func TestRangeProofWithInvalidNonExistentProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Root()
	require.NoError(t, err)

	start, end := 100, 200
	first := decrementFelt(records[start].key)

	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(first, records[end-1].key, proof)
	require.NoError(t, err)

	start = 105 // Gap created
	keys := make([]*felt.Felt, end-start)
	values := make([]*felt.Felt, end-start)
	for i := start; i < end; i++ {
		keys[i-start] = records[i].key
		values[i-start] = records[i].value
	}

	_, err = trie.VerifyRangeProof(root, first, keys, values, proof)
	require.Error(t, err)
}

func TestOneElementRangeProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Root()
	require.NoError(t, err)

	t.Run("both edge proofs with the same key", func(t *testing.T) {
		t.Parallel()

		start := 100
		proof := trie.NewProofNodeSet()
		err = tr.GetRangeProof(records[start].key, records[start].key, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(root, records[start].key, []*felt.Felt{records[start].key}, []*felt.Felt{records[start].value}, proof)
		require.NoError(t, err)
	})

	t.Run("left non-existent edge proof", func(t *testing.T) {
		t.Parallel()

		start := 100
		proof := trie.NewProofNodeSet()
		err = tr.GetRangeProof(decrementFelt(records[start].key), records[start].key, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(root, decrementFelt(records[start].key), []*felt.Felt{records[start].key}, []*felt.Felt{records[start].value}, proof)
		require.NoError(t, err)
	})

	t.Run("right non-existent edge proof", func(t *testing.T) {
		t.Parallel()

		end := 100
		proof := trie.NewProofNodeSet()
		err = tr.GetRangeProof(records[end].key, incrementFelt(records[end].key), proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(root, records[end].key, []*felt.Felt{records[end].key}, []*felt.Felt{records[end].value}, proof)
		require.NoError(t, err)
	})

	t.Run("both non-existent edge proofs", func(t *testing.T) {
		t.Parallel()

		start := 100
		first, last := decrementFelt(records[start].key), incrementFelt(records[start].key)
		proof := trie.NewProofNodeSet()
		err = tr.GetRangeProof(first, last, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(root, first, []*felt.Felt{records[start].key}, []*felt.Felt{records[start].value}, proof)
		require.NoError(t, err)
	})

	t.Run("1 key trie", func(t *testing.T) {
		t.Parallel()

		tr, records := build1KeyTrie(t)
		root, err := tr.Root()
		require.NoError(t, err)

		proof := trie.NewProofNodeSet()
		err = tr.GetRangeProof(&felt.Zero, records[0].key, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(root, records[0].key, []*felt.Felt{records[0].key}, []*felt.Felt{records[0].value}, proof)
		require.NoError(t, err)
	})
}

// TestAllElementsProof tests the range proof with all elements and nil proof.
func TestAllElementsRangeProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Root()
	require.NoError(t, err)

	keys := make([]*felt.Felt, n)
	values := make([]*felt.Felt, n)
	for i, record := range records {
		keys[i] = record.key
		values[i] = record.value
	}

	_, err = trie.VerifyRangeProof(root, nil, keys, values, nil)
	require.NoError(t, err)

	// Should also work with proof
	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(records[0].key, records[n-1].key, proof)
	require.NoError(t, err)

	_, err = trie.VerifyRangeProof(root, keys[0], keys, values, proof)
	require.NoError(t, err)
}

// TestSingleSideRangeProof tests the range proof starting with zero.
func TestSingleSideRangeProof(t *testing.T) {
	t.Parallel()

	tr, records := randomTrie(t, 1000)
	root, err := tr.Root()
	require.NoError(t, err)

	for i := 0; i < len(records); i += 100 {
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(&felt.Zero, records[i].key, proof)
		require.NoError(t, err)

		keys := make([]*felt.Felt, i+1)
		values := make([]*felt.Felt, i+1)
		for j := 0; j < i+1; j++ {
			keys[j] = records[j].key
			values[j] = records[j].value
		}

		_, err = trie.VerifyRangeProof(root, &felt.Zero, keys, values, proof)
		require.NoError(t, err)
	}
}

// TODO(weiihann): gapped range proof sometime succeeds, as the way we handle
func TestGappedRangeProof(t *testing.T) {
	t.Parallel()

	tr, records := nonRandomTrie(t, 10)
	root, err := tr.Root()
	require.NoError(t, err)

	first, last := 2, 8
	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(records[first].key, records[last].key, proof)
	require.NoError(t, err)

	keys := []*felt.Felt{}
	values := []*felt.Felt{}
	for i := first; i <= last; i++ {
		if i == (first+last)/2 {
			continue
		}

		keys = append(keys, records[i].key)
		values = append(values, records[i].value)
	}

	_, err = trie.VerifyRangeProof(root, records[first].key, keys, values, proof)
	require.Error(t, err)
}

// TestBadRangeProof generates random bad proof scenarios and verifies that the proof is invalid.
func TestBadRangeProof(t *testing.T) {
	t.Parallel()

	tr, records := randomTrie(t, 1000)
	root, err := tr.Root()
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		start := rand.Intn(len(records))
		end := rand.Intn(len(records)-start) + start + 1

		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[start].key, records[end-1].key, proof)
		require.NoError(t, err)

		keys := []*felt.Felt{}
		values := []*felt.Felt{}
		for j := start; j < end; j++ {
			keys = append(keys, records[j].key)
			values = append(values, records[j].value)
		}

		first := keys[0]
		testCase := rand.Intn(5)

		index := rand.Intn(end - start)
		switch testCase {
		case 0: // modified key
			keys[index] = new(felt.Felt).SetUint64(rand.Uint64())
		case 1: // modified value
			values[index] = new(felt.Felt).SetUint64(rand.Uint64())
		case 2: // out of order
			index2 := rand.Intn(end - start)
			if index2 == index {
				continue
			}
			keys[index], keys[index2] = keys[index2], keys[index]
			values[index], values[index2] = values[index2], values[index]
		case 3: // set random key to empty
			keys[index] = &felt.Zero
		case 4: // set random value to empty
			values[index] = &felt.Zero
			// TODO(weiihann): gapped kvs sometime succeed, need to investigate further
			// case 5: // gapped
			// 	if end-start < 100 || index == 0 || index == end-start-1 {
			// 		continue
			// 	}
			// 	keys = append(keys[:index], keys[index+1:]...)
			// 	values = append(values[:index], values[index+1:]...)
		}
		_, err = trie.VerifyRangeProof(root, first, keys, values, proof)
		if err == nil {
			t.Fatalf("expected error for test case %d, index %d, start %d, end %d", testCase, index, start, end)
		}
	}
}

func TestRangeProof4KeysTrieD(t *testing.T) {
	tr, records := build4KeysTrieD(t)
	root, err := tr.Root()
	require.NoError(t, err)
	t.Run("start key from zero", func(t *testing.T) {
		for i := 0; i < len(records); i++ {
			proof := trie.NewProofNodeSet()
			err := tr.GetRangeProof(&felt.Zero, records[i].key, proof)
			require.NoError(t, err)

			keys := make([]*felt.Felt, i+1)
			values := make([]*felt.Felt, i+1)
			for j := 0; j < i+1; j++ {
				keys[j] = records[j].key
				values[j] = records[j].value
			}

			hasMore, err := trie.VerifyRangeProof(root, &felt.Zero, keys, values, proof)
			require.NoError(t, err)
			if i == len(records)-1 {
				require.False(t, hasMore)
			} else {
				require.True(t, hasMore)
			}
		}
	})

	t.Run("one existent element proof", func(t *testing.T) {
		for i, record := range records {
			proof := trie.NewProofNodeSet()
			err := tr.GetRangeProof(record.key, record.key, proof)
			require.NoError(t, err)

			keys := make([]*felt.Felt, 1)
			values := make([]*felt.Felt, 1)
			keys[0] = record.key
			values[0] = record.value

			hasMore, err := trie.VerifyRangeProof(root, record.key, keys, values, proof)
			require.NoError(t, err)
			if i == len(records)-1 {
				require.False(t, hasMore)
			} else {
				require.True(t, hasMore)
			}
		}
	})

	t.Run("all existent elements proof", func(t *testing.T) {
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[0].key, records[len(records)-1].key, proof)
		require.NoError(t, err)

		root, err := tr.Root()
		require.NoError(t, err)

		keys := []*felt.Felt{}
		values := []*felt.Felt{}
		for _, record := range records {
			keys = append(keys, record.key)
			values = append(values, record.value)
		}

		hasMore, err := trie.VerifyRangeProof(root, records[0].key, keys, values, proof)
		require.NoError(t, err)
		require.False(t, hasMore)
	})
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
			tr, _ := scenario.buildFn(t)

			for _, tc := range scenario.testKeys {
				t.Run(tc.name, func(t *testing.T) {
					proofSet := trie.NewProofNodeSet()
					err := tr.Prove(tc.key, proofSet)
					require.NoError(t, err)

					nodes := trie.NewStorageNodeSet()
					root, err := tr.Root()
					require.NoError(t, err)
					keyFelt := tr.FeltToKey(tc.key)
					rootKey, _, err := trie.ProofToPath(root, &keyFelt, proofSet, nodes)
					require.NoError(t, err)

					tr2, err := trie.BuildTrie(scenario.height, rootKey, nodes.List(), nil, nil)
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
	tempTrie, records := nonRandomTrie(t, n)

	for _, record := range records {
		key := tempTrie.FeltToKey(record.key)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		nodes := trie.NewStorageNodeSet()
		rootKey, _, err := trie.ProofToPath(root, &key, proofSet, nodes)
		require.NoError(t, err)

		tr2, err := trie.BuildTrie(251, rootKey, nodes.List(), nil, nil)
		require.NoError(t, err)

		root2, err := tr2.Root()
		require.NoError(t, err)
		require.Equal(t, root, root2)

		// Verify value
		value, err := tr2.Get(record.key)
		require.NoError(t, err)
		require.Equal(t, record.value, value)
	}
}

func TestProofToPathNKeysWithNonExistentKeys(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie, records := nonRandomTrie(t, n)

	for _, record := range records {
		key := tempTrie.FeltToKey(record.key)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		nodes := trie.NewStorageNodeSet()
		rootKey, _, err := trie.ProofToPath(root, &key, proofSet, nodes)
		require.NoError(t, err)

		tr2, err := trie.BuildTrie(251, rootKey, nodes.List(), nil, nil)
		require.NoError(t, err)

		root2, err := tr2.Root()
		require.NoError(t, err)
		require.Equal(t, root, root2)
	}
}

func TestProofToPathRandomTrie(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie, records := randomTrie(t, n)

	for _, record := range records {
		key := tempTrie.FeltToKey(record.key)

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		nodes := trie.NewStorageNodeSet()
		rootKey, _, err := trie.ProofToPath(root, &key, proofSet, nodes)
		require.NoError(t, err)

		tr2, err := trie.BuildTrie(251, rootKey, nodes.List(), nil, nil)
		require.NoError(t, err)

		root2, err := tr2.Root()
		require.NoError(t, err)
		require.Equal(t, root, root2)
	}
}

func buildTrie(t *testing.T, height uint8, records []*keyValue) *trie.Trie {
	if len(records) == 0 {
		t.Fatal("records must have at least one element")
	}

	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), height)
	require.NoError(t, err)

	for _, record := range records {
		_, err = tempTrie.Put(record.key, record.value)
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	return tempTrie
}

func build1KeyTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	return nonRandomTrie(t, 1)
}

func buildSimpleTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	//   (250, 0, x1)		edge
	//        |
	//     (0,0,x1)			binary
	//      /    \
	//     (2)  (3)
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(2)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(3)},
	}

	return buildTrie(t, 251, records), records
}

func buildSimpleBinaryRootTrie(t *testing.T) (*trie.Trie, []*keyValue) {
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
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: utils.HexToFelt(t, "0xcc")},
		{key: utils.HexToFelt(t, "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), value: utils.HexToFelt(t, "0xdd")},
	}
	return buildTrie(t, 251, records), records
}

func buildSimpleDoubleBinaryTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	//           (249,0,x3)         // Edge
	//               |
	//           (0, 0, x3)         // Binary
	//         /            \
	//     (0,0,x1) // B  (1, 1, 5) // Edge leaf
	//      /    \             |
	//     (2)  (3)           (5)
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(2)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(3)},
		{key: new(felt.Felt).SetUint64(3), value: new(felt.Felt).SetUint64(5)},
	}
	return buildTrie(t, 251, records), records
}

func build3KeyTrie(t *testing.T) (*trie.Trie, []*keyValue) {
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
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(2), value: new(felt.Felt).SetUint64(6)},
	}

	return buildTrie(t, 251, records), records
}

func buildStarknetDocsTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(2), value: new(felt.Felt).SetUint64(1)},
		{key: new(felt.Felt).SetUint64(5), value: new(felt.Felt).SetUint64(1)},
	}
	return buildTrie(t, 3, records), records
}

func build4KeysTrieA(t *testing.T) (*trie.Trie, []*keyValue) {
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
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(2), value: new(felt.Felt).SetUint64(6)},
		{key: new(felt.Felt).SetUint64(4), value: new(felt.Felt).SetUint64(7)},
	}
	return buildTrie(t, 251, records), records
}

func build4KeysTrieB(t *testing.T) (*trie.Trie, []*keyValue) {
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(2), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(3), value: new(felt.Felt).SetUint64(6)},
		{key: new(felt.Felt).SetUint64(4), value: new(felt.Felt).SetUint64(7)},
	}
	return buildTrie(t, 251, records), records
}

func build4KeysTrieC(t *testing.T) (*trie.Trie, []*keyValue) {
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(4), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(5), value: new(felt.Felt).SetUint64(6)},
		{key: new(felt.Felt).SetUint64(7), value: new(felt.Felt).SetUint64(7)},
	}
	return buildTrie(t, 251, records), records
}

func build4KeysTrieD(t *testing.T) (*trie.Trie, []*keyValue) {
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(4), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(6), value: new(felt.Felt).SetUint64(6)},
		{key: new(felt.Felt).SetUint64(7), value: new(felt.Felt).SetUint64(7)},
	}
	return buildTrie(t, 251, records), records
}

func nonRandomTrie(t *testing.T, numKeys int) (*trie.Trie, []*keyValue) {
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)

	records := make([]*keyValue, numKeys)
	for i := 1; i < numKeys+1; i++ {
		key := new(felt.Felt).SetUint64(uint64(i))
		records[i-1] = &keyValue{key: key, value: key}
		_, err := tempTrie.Put(key, key)
		require.NoError(t, err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].key.Cmp(records[j].key) < 0
	})

	require.NoError(t, tempTrie.Commit())

	return tempTrie, records
}

func randomTrie(t *testing.T, n int) (*trie.Trie, []*keyValue) {
	rrand := rand.New(rand.NewSource(3))

	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)

	records := make([]*keyValue, n)
	for i := 0; i < n; i++ {
		key := new(felt.Felt).SetUint64(uint64(rrand.Uint32() + 1))
		records[i] = &keyValue{key: key, value: key}
		_, err := tempTrie.Put(key, key)
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	// Sort records by key
	sort.Slice(records, func(i, j int) bool {
		return records[i].key.Cmp(records[j].key) < 0
	})

	return tempTrie, records
}

// func incrementKey(k *trie.Key) trie.Key {
// 	var bigInt big.Int

// 	keyBytes := k.Bytes()
// 	bigInt.SetBytes(keyBytes[:])
// 	bigInt.Add(&bigInt, big.NewInt(1))
// 	bigInt.FillBytes(keyBytes[:])
// 	return trie.NewKey(k.Len()+1, keyBytes[:])
// }

// func decrementKey(k *trie.Key) trie.Key {
// 	var bigInt big.Int

// 	keyBytes := k.Bytes()
// 	bigInt.SetBytes(keyBytes[:])
// 	bigInt.Sub(&bigInt, big.NewInt(1))
// 	bigInt.FillBytes(keyBytes[:])
// 	return trie.NewKey(k.Len()-1, keyBytes[:])
// }

func decrementFelt(f *felt.Felt) *felt.Felt {
	return new(felt.Felt).Sub(f, new(felt.Felt).SetUint64(1))
}

func incrementFelt(f *felt.Felt) *felt.Felt {
	return new(felt.Felt).Add(f, new(felt.Felt).SetUint64(1))
}

type keyValue struct {
	key   *felt.Felt
	value *felt.Felt
}
