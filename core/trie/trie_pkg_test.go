package trie

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrieKeys(t *testing.T) {
	t.Run("put to empty trie", func(t *testing.T) {
		tempTrie, err := NewTriePedersen(newMemStorage(), 251)
		require.NoError(t, err)
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

	t.Run("put a left then a right node", func(t *testing.T) {
		tempTrie, err := NewTriePedersen(newMemStorage(), 251)
		require.NoError(t, err)
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
		tempTrie, err := NewTriePedersen(newMemStorage(), 251)
		require.NoError(t, err)
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
		tempTrie, err := NewTriePedersen(newMemStorage(), 251)
		require.NoError(t, err)
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

func TestTrieKeysAfterDeleteSubtree(t *testing.T) {
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
			tempTrie, err := NewTriePedersen(newMemStorage(), 251)
			require.NoError(t, err)
			// Build a basic trie
			_, err = tempTrie.Put(leftLeftKey, leftLeftVal)
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
