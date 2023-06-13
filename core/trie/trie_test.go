package trie_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
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
		require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
			key := new(felt.Felt).SetUint64(1)
			zeroVal := new(felt.Felt).SetUint64(0)

			oldVal, err := tempTrie.Put(key, zeroVal)
			require.NoError(t, err)

			assert.Nil(t, oldVal)

			return nil
		}))
	})

	t.Run("put zero value", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			zeroVal := new(felt.Felt).SetUint64(0)

			_, err = tempTrie.Put(key, zeroVal)
			require.NoError(t, err)

			value, err := tempTrie.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, &felt.Zero, value)
			// Trie's root should be nil
			assert.Nil(t, tempTrie.RootKey())

			return nil
		}))
	})

	t.Run("put to replace an existed value", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
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

			assert.Equal(t, newVal, value)

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
			require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
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
					assert.Equal(t, &felt.Zero, val, "should return zero value when access a deleted key")
				}

				// Check the final rootKey

				if test.expectRootKey != nil {
					expectKeyBytes := test.expectRootKey.Bits()
					assert.Equal(t, expectKeyBytes[:], tempTrie.RootKey().Bytes())
				} else {
					assert.Nil(t, tempTrie.RootKey())
				}

				return nil
			}))
		})
	}
}

func TestPutZero(t *testing.T) {
	require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
		emptyRoot, err := tempTrie.Root()
		require.NoError(t, err)
		var roots []*felt.Felt
		var keys []*felt.Felt

		// put random 64 keys and record roots
		for i := 0; i < 64; i++ {
			key, value := new(felt.Felt), new(felt.Felt)

			_, err = key.SetRandom()
			require.NoError(t, err)

			_, err = value.SetRandom()
			require.NoError(t, err)

			_, err = tempTrie.Put(key, value)
			require.NoError(t, err)

			keys = append(keys, key)

			var root *felt.Felt
			root, err = tempTrie.Root()
			require.NoError(t, err)

			roots = append(roots, root)
		}

		t.Run("adding a zero value to a non-existent key should not change Trie", func(t *testing.T) {
			var key, root *felt.Felt
			key, err = new(felt.Felt).SetRandom()
			require.NoError(t, err)

			_, err = tempTrie.Put(key, new(felt.Felt))
			require.NoError(t, err)

			root, err = tempTrie.Root()
			require.NoError(t, err)

			assert.Equal(t, true, root.Equal(roots[len(roots)-1]))
		})

		t.Run("remove keys one by one, check roots", func(t *testing.T) {
			var gotRoot *felt.Felt
			// put zero in reverse order and check roots still match
			for i := 0; i < 64; i++ {
				root := roots[len(roots)-1-i]

				gotRoot, err = tempTrie.Root()
				require.NoError(t, err)

				assert.Equal(t, root, gotRoot)

				key := keys[len(keys)-1-i]
				_, err = tempTrie.Put(key, new(felt.Felt))
				require.NoError(t, err)
			}
		})

		t.Run("empty roots should match", func(t *testing.T) {
			actualEmptyRoot, err := tempTrie.Root()
			require.NoError(t, err)

			assert.Equal(t, true, actualEmptyRoot.Equal(emptyRoot))
		})
		return nil
	}))
}

func TestOldData(t *testing.T) {
	require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
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
		assert.Error(t, trie.RunOnTempTrie(felt.Bits+1, func(_ *trie.Trie) error {
			return nil
		}))
	})

	t.Run("insert invalid key", func(t *testing.T) {
		height := uint64(felt.Bits)
		require.NoError(t, trie.RunOnTempTrie(uint(height), func(tt *trie.Trie) error {
			badKey := new(felt.Felt).Sub(&felt.Zero, new(felt.Felt).SetUint64(1))
			_, err := tt.Put(badKey, new(felt.Felt))
			assert.Error(t, err)
			return nil
		}))
	})
}

func BenchmarkTriePut(b *testing.B) {
	keys := make([]*felt.Felt, 0, b.N)
	for i := 0; i < b.N; i++ {
		rnd, err := new(felt.Felt).SetRandom()
		require.NoError(b, err)
		keys = append(keys, rnd)
	}

	one := new(felt.Felt).SetUint64(1)
	require.NoError(b, trie.RunOnTempTrie(251, func(t *trie.Trie) error {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := t.Put(keys[i], one)
			if err != nil {
				return err
			}
		}
		return t.Commit()
	}))
}
