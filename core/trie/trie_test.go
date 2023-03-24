package trie

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
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
	t.Run("put zero to empty tree", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)

		key := new(felt.Felt).SetUint64(1)
		zeroVal := new(felt.Felt).SetUint64(0)

		oldVal, err := tempTrie.Put(key, zeroVal)
		require.NoError(t, err)

		assert.Nil(t, oldVal)
	})

	t.Run("put to empty tree", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
		keyNum, err := strconv.ParseUint("1101", 2, 64)
		require.NoError(t, err)

		key := new(felt.Felt).SetUint64(keyNum)
		val := new(felt.Felt).SetUint64(11)

		_, err = tempTrie.Put(key, val)
		require.NoError(t, err)

		value, err := tempTrie.Get(key)
		require.NoError(t, err)

		assert.Equal(t, val, value, "key-val not match")
		assert.Equal(t, tempTrie.feltToBitSet(key), tempTrie.rootKey, "root key not match single node's key")
	})

	t.Run("put zero value", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
		keyNum, err := strconv.ParseUint("1101", 2, 64)
		require.NoError(t, err)

		key := new(felt.Felt).SetUint64(keyNum)
		zeroVal := new(felt.Felt).SetUint64(0)

		_, err = tempTrie.Put(key, zeroVal)
		require.NoError(t, err)

		value, err := tempTrie.Get(key)
		// should return an error when try to access a non-exist key
		assert.Error(t, err)
		// after empty, the return value and Trie's root should be nil
		assert.Nil(t, value)
		assert.Nil(t, tempTrie.rootKey)
	})

	t.Run("put to replace an existed value", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
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
	})

	t.Run("put a left then a right node", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
		// First put a left node
		leftKeyNum, err := strconv.ParseUint("10001", 2, 64)
		require.NoError(t, err)

		leftKey := new(felt.Felt).SetUint64(leftKeyNum)
		leftVal := new(felt.Felt).SetUint64(12)

		_, err = tempTrie.Put(leftKey, leftVal)
		require.NoError(t, err)

		// Then put a right node
		rightKeyNum, err := strconv.ParseUint("10011", 2, 64)
		require.NoError(t, err)

		rightKey := new(felt.Felt).SetUint64(rightKeyNum)
		rightVal := new(felt.Felt).SetUint64(22)

		_, err = tempTrie.Put(rightKey, rightVal)
		require.NoError(t, err)

		// Check parent and its left right children
		commonKey, isSame := findCommonKey(tempTrie.feltToBitSet(leftKey), tempTrie.feltToBitSet(rightKey))
		require.False(t, isSame)

		// Common key should be 0b100, length 251-2;
		expectKey := bitset.New(251 - 2).Set(2)

		assert.Equal(t, expectKey, commonKey)

		// Current rootKey should be the common key
		assert.Equal(t, expectKey, tempTrie.rootKey)

		parentNode, err := tempTrie.storage.Get(commonKey)
		require.NoError(t, err)

		assert.Equal(t, tempTrie.feltToBitSet(leftKey), parentNode.Left)
		assert.Equal(t, tempTrie.feltToBitSet(rightKey), parentNode.Right)
	})

	t.Run("put a right node then a left node", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
		// First put a right node
		rightKeyNum, err := strconv.ParseUint("10011", 2, 64)
		require.NoError(t, err)

		rightKey := new(felt.Felt).SetUint64(rightKeyNum)
		rightVal := new(felt.Felt).SetUint64(22)
		_, err = tempTrie.Put(rightKey, rightVal)
		require.NoError(t, err)

		// Then put a left node
		leftKeyNum, err := strconv.ParseUint("10001", 2, 64)
		require.NoError(t, err)

		leftKey := new(felt.Felt).SetUint64(leftKeyNum)
		leftVal := new(felt.Felt).SetUint64(12)

		_, err = tempTrie.Put(leftKey, leftVal)
		require.NoError(t, err)

		// Check parent and its left right children
		commonKey, isSame := findCommonKey(tempTrie.feltToBitSet(leftKey), tempTrie.feltToBitSet(rightKey))
		require.False(t, isSame)

		expectKey := bitset.New(251 - 2).Set(2)

		assert.Equal(t, expectKey, commonKey)

		parentNode, err := tempTrie.storage.Get(commonKey)
		require.NoError(t, err)

		assert.Equal(t, tempTrie.feltToBitSet(leftKey), parentNode.Left)
		assert.Equal(t, tempTrie.feltToBitSet(rightKey), parentNode.Right)
	})

	t.Run("Add new key to different branches", func(t *testing.T) {
		tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
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

		// Build a basic trie
		_, err = tempTrie.Put(leftKey, leftVal)
		require.NoError(t, err)

		_, err = tempTrie.Put(rightKey, rightVal)
		require.NoError(t, err)

		t.Run("Add to left branch", func(t *testing.T) {
			newKeyNum, err := strconv.ParseUint("101", 2, 64)
			require.NoError(t, err)

			newKey := new(felt.Felt).SetUint64(newKeyNum)
			newVal := new(felt.Felt).SetUint64(12)

			_, err = tempTrie.Put(newKey, newVal)
			require.NoError(t, err)

			commonKey := bitset.New(251 - 1).Set(1)

			parentNode, err := tempTrie.storage.Get(commonKey)
			require.NoError(t, err)

			assert.Equal(t, tempTrie.feltToBitSet(leftKey), parentNode.Left)
			assert.Equal(t, tempTrie.feltToBitSet(newKey), parentNode.Right)
		})
		t.Run("Add to right branch", func(t *testing.T) {
			newKeyNum, err := strconv.ParseUint("110", 2, 64)
			require.NoError(t, err)

			newKey := new(felt.Felt).SetUint64(newKeyNum)
			newVal := new(felt.Felt).SetUint64(12)

			_, err = tempTrie.Put(newKey, newVal)
			require.NoError(t, err)

			commonKey := bitset.New(251 - 1).Set(0).Set(1)
			parentNode, err := tempTrie.storage.Get(commonKey)
			require.NoError(t, err)

			assert.Equal(t, tempTrie.feltToBitSet(newKey), parentNode.Left)
			assert.Equal(t, tempTrie.feltToBitSet(rightKey), parentNode.Right)
		})
		t.Run("Add new node as parent sibling", func(t *testing.T) {
			newKeyNum, err := strconv.ParseUint("000", 2, 64)
			require.NoError(t, err)

			newKey := new(felt.Felt).SetUint64(newKeyNum)
			newVal := new(felt.Felt).SetUint64(12)

			_, err = tempTrie.Put(newKey, newVal)
			require.NoError(t, err)

			commonKey := bitset.New(251 - 3)
			parentNode, err := tempTrie.storage.Get(commonKey)
			require.NoError(t, err)

			assert.Equal(t, tempTrie.feltToBitSet(newKey), parentNode.Left)

			expectRightKey := bitset.New(251 - 2).Set(0)

			assert.Equal(t, expectRightKey, parentNode.Right)
		})
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
			tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
			// Build a basic trie
			_, err := tempTrie.Put(leftKey, leftVal)
			require.NoError(t, err)

			_, err = tempTrie.Put(rightKey, rightVal)
			require.NoError(t, err)

			for _, key := range test.deleteKeys {
				_, err := tempTrie.Put(key, zeroVal)
				require.NoError(t, err)

				val, err := tempTrie.Get(key)

				assert.Error(t, err, "should return an error when access a deleted key")
				assert.Nil(t, val, "should return an nil value when access a deleted key")
			}

			// Check the final rootKey
			assert.Equal(t, tempTrie.feltToBitSet(test.expectRootKey), tempTrie.rootKey)
		})
	}
}

func TestTrieDeleteSubtree(t *testing.T) {
	// Left branch's left child
	leftLeftKeyNum, err := strconv.ParseUint("100", 2, 64)
	require.NoError(t, err)

	leftLeftKey := new(felt.Felt).SetUint64(leftLeftKeyNum)
	leftLeftVal := new(felt.Felt).SetUint64(11)

	// Left branch's right child
	leftRightKeyNum, err := strconv.ParseUint("101", 2, 64)
	require.NoError(t, err)

	leftRightKey := new(felt.Felt).SetUint64(leftRightKeyNum)
	leftRightVal := new(felt.Felt).SetUint64(22)

	// Right branch's node
	rightKeyNum, err := strconv.ParseUint("111", 2, 64)
	require.NoError(t, err)

	rightKey := new(felt.Felt).SetUint64(rightKeyNum)
	rightVal := new(felt.Felt).SetUint64(33)

	// Zero value
	zeroVal := new(felt.Felt).SetUint64(0)

	tests := [...]struct {
		name       string
		deleteKey  *felt.Felt
		expectLeft *felt.Felt
	}{
		{
			name:       "delete the left branch's left child",
			deleteKey:  leftLeftKey,
			expectLeft: leftRightKey,
		},
		{
			name:       "delete the left branch's right child",
			deleteKey:  leftRightKey,
			expectLeft: leftLeftKey,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
			// Build a basic trie
			_, err := tempTrie.Put(leftLeftKey, leftLeftVal)
			require.NoError(t, err)

			_, err = tempTrie.Put(leftRightKey, leftRightVal)
			require.NoError(t, err)

			_, err = tempTrie.Put(rightKey, rightVal)
			require.NoError(t, err)

			// Delete the node on left sub branch
			_, err = tempTrie.Put(test.deleteKey, zeroVal)
			require.NoError(t, err)

			newRootKey := bitset.New(251 - 2).Set(0)

			assert.Equal(t, newRootKey, tempTrie.rootKey)

			rootNode, err := tempTrie.storage.Get(newRootKey)
			require.NoError(t, err)

			assert.Equal(t, tempTrie.feltToBitSet(rightKey), rootNode.Right)
			assert.Equal(t, tempTrie.feltToBitSet(test.expectLeft), rootNode.Left)
		})
	}
}

func TestPutZero(t *testing.T) {
	storage := newMemStorage()
	trie := NewTriePedersen(storage, 251, nil)
	emptyRoot, err := trie.Root()
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

		_, err = trie.Put(key, value)
		require.NoError(t, err)

		keys = append(keys, key)

		var root *felt.Felt
		root, err = trie.Root()
		require.NoError(t, err)

		roots = append(roots, root)
	}

	key, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	// adding a zero value should not change Trie
	_, err = trie.Put(key, new(felt.Felt))
	require.NoError(t, err)

	root, err := trie.Root()
	require.NoError(t, err)

	assert.Equal(t, true, root.Equal(roots[len(roots)-1]))

	var gotRoot *felt.Felt
	// put zero in reverse order and check roots still match
	for i := 0; i < 64; i++ {
		root = roots[len(roots)-1-i]

		gotRoot, err = trie.Root()
		require.NoError(t, err)

		assert.Equal(t, root, gotRoot)

		key := keys[len(keys)-1-i]
		_, err = trie.Put(key, new(felt.Felt))
		require.NoError(t, err)
	}

	actualEmptyRoot, err := trie.Root()
	require.NoError(t, err)

	assert.Equal(t, true, actualEmptyRoot.Equal(emptyRoot))
	assert.Zero(t, len(storage.storage)) // storage should be empty
}

func TestOldData(t *testing.T) {
	tempTrie := NewTriePedersen(newMemStorage(), 251, nil)
	key := new(felt.Felt).SetUint64(12)
	old := new(felt.Felt)

	was, err := tempTrie.Put(key, old)
	require.NoError(t, err)

	assert.Nil(t, was) // no change

	was, err = tempTrie.Put(key, new(felt.Felt).SetUint64(1))
	require.NoError(t, err)

	assert.Equal(t, old, was)

	old.SetUint64(1)

	was, err = tempTrie.Put(key, new(felt.Felt).SetUint64(2))
	require.NoError(t, err)

	assert.Equal(t, old, was)

	old.SetUint64(2)

	// put zero value to delete current key
	was, err = tempTrie.Put(key, new(felt.Felt))
	require.NoError(t, err)

	assert.Equal(t, old, was)

	// put zero again to check old data
	was, err = tempTrie.Put(key, new(felt.Felt))
	require.NoError(t, err)

	// there should no old data to return
	assert.Nil(t, was)
}
