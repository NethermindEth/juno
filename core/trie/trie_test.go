package trie_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Todo: Refactor:
//
//   - [*] Test names should not have "_"
//   - [*] Table test are being used incorrectly: they should be separated into subsets, see node_test.go
//   - [*] Functions such as Path and findCommonKey don't need to be public. Thus,
//     they don't need to be tested explicitly.
//   - [*] There are warning which ignore returned errors, returned errors should not be ignored.
//   - [ ] Add more test cases with different heights
//   - [*] Add more complicated Put and Delete scenarios
func TestTriePut(t *testing.T) {
	t.Run("put zero to empty trie", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
			key := new(felt.Felt).SetUint64(1)
			zeroVal := new(felt.Felt).SetUint64(0)

			oldVal, err := tempTrie.Put(key, zeroVal)
			require.NoError(t, err)

			assert.Nil(t, oldVal)

			return nil
		}))
	})

	t.Run("put zero value", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			zeroVal := new(felt.Felt).SetUint64(0)

			_, err = tempTrie.Put(key, zeroVal)
			require.NoError(t, err)

			value, err := tempTrie.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, felt.Zero, value)
			// Trie's root should be nil
			assert.Nil(t, tempTrie.RootKey())

			return nil
		}))
	})

	t.Run("put to replace an existed value", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			val := new(felt.Felt).SetUint64(1)

			_, err = tempTrie.Put(key, val)
			require.NoError(t, err)

			newVal := new(felt.Felt).SetUint64(2)

			_, err = tempTrie.Put(key, newVal)
			require.NoError(t, err, "update a new value at an exist key")

			value, err := tempTrie.Get(key)
			require.NoError(t, err)

			assert.Equal(t, newVal, &value)

			return nil
		}))
	})
}

func TestTrieDeleteBasic(t *testing.T) {
	// left branch
	leftKeyNum, err := strconv.ParseUint("100", 2, 64)
	require.NoError(t, err)

	leftKey := new(felt.Felt).SetUint64(leftKeyNum)
	leftVal := new(felt.Felt).SetUint64(12)

	// right branch
	rightKeyNum, err := strconv.ParseUint("111", 2, 64)
	require.NoError(t, err)

	rightKey := new(felt.Felt).SetUint64(rightKeyNum)
	rightVal := new(felt.Felt).SetUint64(22)

	// Zero value
	zeroVal := new(felt.Felt).SetUint64(0)

	tests := [...]struct {
		name          string
		deleteKeys    []*felt.Felt
		expectRootKey *felt.Felt
	}{
		{
			name:          "delete left child",
			deleteKeys:    []*felt.Felt{leftKey},
			expectRootKey: rightKey,
		},
		{
			name:          "delete right child",
			deleteKeys:    []*felt.Felt{rightKey},
			expectRootKey: leftKey,
		},
		{
			name:          "delete both children",
			deleteKeys:    []*felt.Felt{leftKey, rightKey},
			expectRootKey: (*felt.Felt)(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
				// Build a basic trie
				_, err := tempTrie.Put(leftKey, leftVal)
				require.NoError(t, err)

				_, err = tempTrie.Put(rightKey, rightVal)
				require.NoError(t, err)

				for _, key := range test.deleteKeys {
					_, err := tempTrie.Put(key, zeroVal)
					require.NoError(t, err)

					val, err := tempTrie.Get(key)

					assert.NoError(t, err, "shouldnt return an error when access a deleted key")
					assert.Equal(t, felt.Zero, val, "should return zero value when access a deleted key")
				}

				// Check the final rootKey

				if test.expectRootKey != nil {
					assert.Equal(t, *test.expectRootKey, tempTrie.RootKey().Felt())
				} else {
					assert.Nil(t, tempTrie.RootKey())
				}

				return nil
			}))
		})
	}
}

func TestPutZero(t *testing.T) {
	require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
		emptyRoot, err := tempTrie.Hash()
		require.NoError(t, err)
		var roots []*felt.Felt
		var keys []*felt.Felt

		// put random 64 keys and record roots
		for range 64 {
			key := felt.NewRandom[felt.Felt]()
			value := felt.NewRandom[felt.Felt]()

			_, err = tempTrie.Put(key, value)
			require.NoError(t, err)

			keys = append(keys, key)

			var root felt.Felt
			root, err = tempTrie.Hash()
			require.NoError(t, err)

			roots = append(roots, &root)
		}

		t.Run(
			"adding a zero value to a non-existent key should not change Trie",
			func(t *testing.T) {
				key := felt.NewRandom[felt.Felt]()

				_, err = tempTrie.Put(key, new(felt.Felt))
				require.NoError(t, err)

				root, err := tempTrie.Hash()
				require.NoError(t, err)

				assert.Equal(t, true, root.Equal(roots[len(roots)-1]))
			})

		t.Run("remove keys one by one, check roots", func(t *testing.T) {
			var gotRoot felt.Felt
			// put zero in reverse order and check roots still match
			for i := range 64 {
				root := roots[len(roots)-1-i]

				gotRoot, err = tempTrie.Hash()
				require.NoError(t, err)

				assert.Equal(t, root, &gotRoot)

				key := keys[len(keys)-1-i]
				_, err = tempTrie.Put(key, new(felt.Felt))
				require.NoError(t, err)
			}
		})

		t.Run("empty roots should match", func(t *testing.T) {
			actualEmptyRoot, err := tempTrie.Hash()
			require.NoError(t, err)

			assert.Equal(t, true, actualEmptyRoot.Equal(&emptyRoot))
		})
		return nil
	}))
}

func TestTrie(t *testing.T) {
	require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
		emptyRoot, err := tempTrie.Hash()
		require.NoError(t, err)
		var roots []*felt.Felt
		var keys []*felt.Felt

		// put random 64 keys and record roots
		for range 64 {
			key := felt.NewRandom[felt.Felt]()
			value := felt.NewRandom[felt.Felt]()

			_, err = tempTrie.Put(key, value)
			require.NoError(t, err)

			keys = append(keys, key)

			var root felt.Felt
			root, err = tempTrie.Hash()
			require.NoError(t, err)

			roots = append(roots, &root)
		}

		t.Run("adding a zero value to a non-existent key should not change Trie", func(t *testing.T) {
			key := felt.NewRandom[felt.Felt]()

			_, err = tempTrie.Put(key, new(felt.Felt))
			require.NoError(t, err)

			root, err := tempTrie.Hash()
			require.NoError(t, err)

			assert.Equal(t, true, (&root).Equal(roots[len(roots)-1]))
		})

		t.Run("remove keys one by one, check roots", func(t *testing.T) {
			var gotRoot felt.Felt
			// put zero in reverse order and check roots still match
			for i := range 64 {
				root := roots[len(roots)-1-i]

				gotRoot, err = tempTrie.Hash()
				require.NoError(t, err)

				assert.Equal(t, root, &gotRoot)

				key := keys[len(keys)-1-i]
				_, err = tempTrie.Put(key, new(felt.Felt))
				require.NoError(t, err)
			}
		})

		t.Run("empty roots should match", func(t *testing.T) {
			actualEmptyRoot, err := tempTrie.Hash()
			require.NoError(t, err)

			assert.Equal(t, true, actualEmptyRoot.Equal(&emptyRoot))
		})
		return nil
	}))
}

func TestOldData(t *testing.T) {
	require.NoError(t, trie.RunOnTempTriePedersen(251, func(tempTrie *trie.Trie) error {
		key := new(felt.Felt).SetUint64(12)
		old := new(felt.Felt)

		t.Run("put zero to empty key, expect no change", func(t *testing.T) {
			was, err := tempTrie.Put(key, old)
			require.NoError(t, err)
			assert.Nil(t, was) // no change
		})

		t.Run("put non-zero to empty key, expect zero", func(t *testing.T) {
			was, err := tempTrie.Put(key, old)
			require.NoError(t, err)
			assert.Nil(t, was) // no change

			newVal := new(felt.Felt).SetUint64(1)
			was, err = tempTrie.Put(key, newVal)
			require.NoError(t, err)

			assert.Equal(t, old, was)
			old.Set(newVal)
		})

		t.Run("change value of a key, expect old value", func(t *testing.T) {
			newVal := new(felt.Felt).SetUint64(2)
			was, err := tempTrie.Put(key, newVal)
			require.NoError(t, err)

			assert.Equal(t, old, was)
			old.Set(newVal)
		})

		t.Run("delete key, expect old value", func(t *testing.T) {
			// put zero value to delete current key
			was, err := tempTrie.Put(key, &felt.Zero)
			require.NoError(t, err)

			assert.Equal(t, old, was)
		})

		t.Run("delete non-existent key, expect no change", func(t *testing.T) {
			// put zero again to check old data
			was, err := tempTrie.Put(key, new(felt.Felt))
			require.NoError(t, err)

			// there should no old data to return
			assert.Nil(t, was)
		})

		return nil
	}))
}

func TestMaxTrieHeight(t *testing.T) {
	t.Run("create trie with invalid height", func(t *testing.T) {
		assert.Error(t, trie.RunOnTempTriePedersen(felt.Bits+1, func(_ *trie.Trie) error {
			return nil
		}))
	})

	t.Run("insert invalid key", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTriePedersen(uint8(felt.Bits), func(tt *trie.Trie) error {
			badKey := new(felt.Felt).Sub(&felt.Zero, new(felt.Felt).SetUint64(1))
			_, err := tt.Put(badKey, new(felt.Felt))
			assert.Error(t, err)
			return nil
		}))
	})
}

func TestRootKeyAlwaysUpdatedOnCommit(t *testing.T) {
	// Not doing what this test requires--always updating the root key on commit--
	// leads to some tricky errors. For example:
	//
	//  1. A trie is created and performs the following operations:
	//     a. Put leaf
	//     b. Commit
	//     c. Delete leaf
	//     d. Commit
	//  2. A second trie is created with the same db transaction and immediately
	//     calls [trie.Root].
	//
	// If the root key is not updated in the db transaction at step 1d,
	// the second trie will initialise its root key to the wrong value
	// (to the value the root key had at step 1b).

	// We simulate the situation described above.

	height := uint8(251)

	// The database transaction we will use to create both tries.
	memDB := memory.New()
	txn := memDB.NewIndexedBatch()

	// Step 1: Create first trie
	tempTrie, err := trie.NewTriePedersen(txn, []byte{1, 2, 3}, height)
	require.NoError(t, err)

	// Step 1a: Put
	key := new(felt.Felt).SetUint64(1)
	_, err = tempTrie.Put(key, new(felt.Felt).SetUint64(1))
	require.NoError(t, err)

	// Step 1b: Commit
	require.NoError(t, tempTrie.Commit())

	// Step 1c: Delete
	_, err = tempTrie.Put(key, new(felt.Felt)) // Inserting zero felt is a deletion.
	require.NoError(t, err)

	want := new(felt.Felt)

	// Step 1d: Commit
	got, err := tempTrie.Hash()
	require.NoError(t, err)
	// Ensure root value matches expectation.
	assert.Equal(t, want, &got)

	// Step 2: Different trie created with the same db transaction and calls [trie.Root].
	secondTrie, err := trie.NewTriePedersen(txn, []byte{1, 2, 3}, height)
	require.NoError(t, err)
	got, err = secondTrie.Hash()
	require.NoError(t, err)
	// Ensure root value is the same as the first trie.
	assert.Equal(t, want, &got)
}

var benchTriePutR *felt.Felt

func BenchmarkTriePut(b *testing.B) {
	keys := make([]*felt.Felt, 0, b.N)
	for range b.N {
		rnd := felt.NewRandom[felt.Felt]()
		keys = append(keys, rnd)
	}

	one := new(felt.Felt).SetUint64(1)
	require.NoError(b, trie.RunOnTempTriePedersen(251, func(t *trie.Trie) error {
		var f *felt.Felt
		var err error
		b.ResetTimer()
		for i := range b.N {
			f, err = t.Put(keys[i], one)
			if err != nil {
				return err
			}
		}
		benchTriePutR = f
		return t.Commit()
	}))
}

// ConcurrentReadsWithinHash tests that Trie.Hash()
// can handle concurrent reads when updateChildTriesConcurrently
// is called.
func TestTrie_Hash_ConcurrentReadsWithinHash(t *testing.T) {
	db, err := pebblev2.New(t.TempDir())
	require.NoError(t, err)

	txn := db.NewIndexedBatch()
	prefix := []byte("test")

	trieInstance, err := trie.NewTriePedersen(txn, prefix, 8)
	require.NoError(t, err)

	for i := range 100 {
		key := felt.NewFromUint64[felt.Felt](uint64(i))
		value := felt.NewFromUint64[felt.Felt](uint64(i * 2))
		_, err := trieInstance.Put(key, value)
		require.NoError(t, err)
	}

	_, err = trieInstance.Hash()
	require.NoError(t, err)
	require.NoError(t, txn.Write())
}

// ConcurrentReadsWithinHash tests that Trie.Hash()
// can handle concurrent reads when updateChildTriesConcurrently
// is called with BufferBatch.
func TestTrie_Hash_ConcurrentReadsWithBufferBatch(t *testing.T) {
	memDB := memory.New()
	baseTxn := memDB.NewIndexedBatch()
	prefix := []byte("test")

	bufferBatch := db.NewBufferBatch(baseTxn)

	trieInstance, err := trie.NewTriePedersen(bufferBatch, prefix, 8)
	require.NoError(t, err)

	for i := range 100 {
		key := felt.NewFromUint64[felt.Felt](uint64(i))
		value := felt.NewFromUint64[felt.Felt](uint64(i * 2))
		_, err := trieInstance.Put(key, value)
		require.NoError(t, err)
	}

	require.NoError(t, bufferBatch.Flush())

	_, err = trieInstance.Hash()
	require.NoError(t, err)
}
