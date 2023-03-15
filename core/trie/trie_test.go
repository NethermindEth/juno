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
		RunOnTempTrie(251, func(trie *Trie) error {
			key := new(felt.Felt).SetUint64(1)
			zeroVal := new(felt.Felt).SetUint64(0)

			oldVal, err := trie.Put(key, zeroVal)
			require.NoError(t, err)

			assert.Nil(t, oldVal)

			return nil
		})
	})
	t.Run("put to empty tree", func(t *testing.T) {
		RunOnTempTrie(251, func(trie *Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			val := new(felt.Felt).SetUint64(11)

			_, err = trie.Put(key, val)
			require.NoError(t, err)

			value, err := trie.Get(key)
			require.NoError(t, err)

			assert.Equal(t, val, value, "key-val not match")
			assert.Equal(t, trie.feltToBitSet(key), trie.rootKey, "root key not match single node's key")

			return nil
		})
	})

	t.Run("put zero value", func(t *testing.T) {
		RunOnTempTrie(251, func(trie *Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			zeroVal := new(felt.Felt).SetUint64(0)

			_, err = trie.Put(key, zeroVal)
			require.NoError(t, err)

			value, err := trie.Get(key)
			// should return an error when try to access a non-exist key
			assert.Error(t, err)
			// after empty, the return value and Trie's root should be nil
			assert.Nil(t, value)
			assert.Nil(t, trie.rootKey)

			return nil
		})
	})
	t.Run("put to replace an existed value", func(t *testing.T) {
		RunOnTempTrie(251, func(trie *Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			val := new(felt.Felt).SetUint64(1)

			_, err = trie.Put(key, val)
			require.NoError(t, err)

			newVal := new(felt.Felt).SetUint64(2)

			_, err = trie.Put(key, newVal)
			require.NoError(t, err, "update a new value at an exist key")

			value, err := trie.Get(key)
			require.NoError(t, err)

			assert.Equal(t, newVal, value)

			return nil
		})
	})
	t.Run("put a left then a right node", func(t *testing.T) {
		RunOnTempTrie(251, func(trie *Trie) error {
			// First put a left node
			leftKeyNum, err := strconv.ParseUint("10001", 2, 64)
			require.NoError(t, err)

			leftKey := new(felt.Felt).SetUint64(leftKeyNum)
			leftVal := new(felt.Felt).SetUint64(12)

			_, err = trie.Put(leftKey, leftVal)
			require.NoError(t, err)

			// Then put a right node
			rightKeyNum, err := strconv.ParseUint("10011", 2, 64)
			require.NoError(t, err)

			rightKey := new(felt.Felt).SetUint64(rightKeyNum)
			rightVal := new(felt.Felt).SetUint64(22)

			_, err = trie.Put(rightKey, rightVal)
			require.NoError(t, err)

			// Check parent and its left right children
			commonKey, isSame := findCommonKey(trie.feltToBitSet(leftKey), trie.feltToBitSet(rightKey))
			require.False(t, isSame)

			// Common key should be 0b100, length 251-2;
			expectKey := bitset.New(251 - 2).Set(2)

			assert.Equal(t, expectKey, commonKey)

			// Current rootKey should be the common key
			assert.Equal(t, expectKey, trie.rootKey)

			parentNode, err := trie.storage.Get(commonKey)
			require.NoError(t, err)

			assert.Equal(t, trie.feltToBitSet(leftKey), parentNode.Left)
			assert.Equal(t, trie.feltToBitSet(rightKey), parentNode.Right)

			return nil
		})
	})
	t.Run("put a right node then a left node", func(t *testing.T) {
		RunOnTempTrie(251, func(trie *Trie) error {
			// First put a right node
			rightKeyNum, err := strconv.ParseUint("10011", 2, 64)
			require.NoError(t, err)

			rightKey := new(felt.Felt).SetUint64(rightKeyNum)
			rightVal := new(felt.Felt).SetUint64(22)
			_, err = trie.Put(rightKey, rightVal)
			require.NoError(t, err)

			// Then put a left node
			leftKeyNum, err := strconv.ParseUint("10001", 2, 64)
			require.NoError(t, err)

			leftKey := new(felt.Felt).SetUint64(leftKeyNum)
			leftVal := new(felt.Felt).SetUint64(12)

			_, err = trie.Put(leftKey, leftVal)
			require.NoError(t, err)

			// Check parent and its left right children
			commonKey, isSame := findCommonKey(trie.feltToBitSet(leftKey), trie.feltToBitSet(rightKey))
			require.False(t, isSame)

			expectKey := bitset.New(251 - 2).Set(2)

			assert.Equal(t, expectKey, commonKey)

			parentNode, err := trie.storage.Get(commonKey)
			require.NoError(t, err)

			assert.Equal(t, trie.feltToBitSet(leftKey), parentNode.Left)
			assert.Equal(t, trie.feltToBitSet(rightKey), parentNode.Right)

			return nil
		})
	})
	t.Run("Add new key to different branches", func(t *testing.T) {
		RunOnTempTrie(251, func(trie *Trie) error {
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
			_, err = trie.Put(leftKey, leftVal)
			require.NoError(t, err)

			_, err = trie.Put(rightKey, rightVal)
			require.NoError(t, err)

			t.Run("Add to left branch", func(t *testing.T) {
				newKeyNum, err := strconv.ParseUint("101", 2, 64)
				require.NoError(t, err)

				newKey := new(felt.Felt).SetUint64(newKeyNum)
				newVal := new(felt.Felt).SetUint64(12)

				_, err = trie.Put(newKey, newVal)
				require.NoError(t, err)

				commonKey := bitset.New(251 - 1).Set(1)

				parentNode, err := trie.storage.Get(commonKey)
				require.NoError(t, err)

				assert.Equal(t, trie.feltToBitSet(leftKey), parentNode.Left)
				assert.Equal(t, trie.feltToBitSet(newKey), parentNode.Right)
			})
			t.Run("Add to right branch", func(t *testing.T) {
				newKeyNum, err := strconv.ParseUint("110", 2, 64)
				require.NoError(t, err)

				newKey := new(felt.Felt).SetUint64(newKeyNum)
				newVal := new(felt.Felt).SetUint64(12)

				_, err = trie.Put(newKey, newVal)
				require.NoError(t, err)

				commonKey := bitset.New(251 - 1).Set(0).Set(1)
				parentNode, err := trie.storage.Get(commonKey)
				require.NoError(t, err)

				assert.Equal(t, trie.feltToBitSet(newKey), parentNode.Left)
				assert.Equal(t, trie.feltToBitSet(rightKey), parentNode.Right)
			})
			t.Run("Add new node as parent sibling", func(t *testing.T) {
				newKeyNum, err := strconv.ParseUint("000", 2, 64)
				require.NoError(t, err)

				newKey := new(felt.Felt).SetUint64(newKeyNum)
				newVal := new(felt.Felt).SetUint64(12)

				_, err = trie.Put(newKey, newVal)
				require.NoError(t, err)

				commonKey := bitset.New(251 - 3)
				parentNode, err := trie.storage.Get(commonKey)
				require.NoError(t, err)

				assert.Equal(t, trie.feltToBitSet(newKey), parentNode.Left)

				expectRightKey := bitset.New(251 - 2).Set(0)

				assert.Equal(t, expectRightKey, parentNode.Right)
			})
			return nil
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
			RunOnTempTrie(251, func(trie *Trie) error {
				// Build a basic trie
				_, err := trie.Put(leftKey, leftVal)
				require.NoError(t, err)

				_, err = trie.Put(rightKey, rightVal)
				require.NoError(t, err)

				for _, key := range test.deleteKeys {
					_, err := trie.Put(key, zeroVal)
					require.NoError(t, err)

					val, err := trie.Get(key)

					assert.Error(t, err, "should return an error when access a deleted key")
					assert.Nil(t, val, "should return an nil value when access a deleted key")
				}

				// Check the final rootKey
				assert.Equal(t, trie.feltToBitSet(test.expectRootKey), trie.rootKey)

				return nil
			})
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
			RunOnTempTrie(251, func(trie *Trie) error {
				// Build a basic trie
				_, err := trie.Put(leftLeftKey, leftLeftVal)
				require.NoError(t, err)

				_, err = trie.Put(leftRightKey, leftRightVal)
				require.NoError(t, err)

				_, err = trie.Put(rightKey, rightVal)
				require.NoError(t, err)

				// Delete the node on left sub branch
				_, err = trie.Put(test.deleteKey, zeroVal)
				require.NoError(t, err)

				newRootKey := bitset.New(251 - 2).Set(0)

				assert.Equal(t, newRootKey, trie.rootKey)

				rootNode, err := trie.storage.Get(newRootKey)
				require.NoError(t, err)

				assert.Equal(t, trie.feltToBitSet(rightKey), rootNode.Right)
				assert.Equal(t, trie.feltToBitSet(test.expectLeft), rootNode.Left)

				return nil
			})
		})
	}
}

func TestPath(t *testing.T) {
	tests := [...]struct {
		parent *bitset.BitSet
		child  *bitset.BitSet
		want   *bitset.BitSet
	}{
		{
			parent: bitset.New(0),
			child:  bitset.New(251).Set(250).Set(249),
			want:   bitset.New(250).Set(249),
		},
		{
			parent: bitset.New(0),
			child:  bitset.New(251).Set(249),
			want:   bitset.New(250).Set(249),
		},
		{
			parent: bitset.New(1).Set(0),
			child:  bitset.New(251).Set(250).Set(249),
			want:   bitset.New(249),
		},
	}

	for idx, test := range tests {
		got := path(test.child, test.parent)
		assert.Equal(t, test.want, got, "TestPath failing #%d", idx)
	}
}

// TestState tests whether the trie produces the same state root as in
// Block 0 of the Starknet protocol mainnet.
func TestState(t *testing.T) {
	// See https://alpha-mainnet.starknet.io/feeder_gateway/get_state_update?blockNumber=0.
	type (
		diff  struct{ key, val string }
		diffs map[string][]diff
	)

	addresses := diffs{
		"0x735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c": {
			{"0x5", "0x64"},
			{
				"0x2f50710449a06a9fa789b3c029a63bd0b1f722f46505828a9f815cf91b31d8",
				"0x2a222e62eabe91abdb6838fa8b267ffe81a6eb575f61e96ec9aa4460c0925a2",
			},
		},
		"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": {
			{"0x5", "0x22b"},
			{
				"0x5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5",
				"0x7e5",
			},
			{
				"0x313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300",
				"0x4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
			},
			{
				"0x313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620301",
				"0x453ae0c9610197b18b13645c44d3d0a407083d96562e8752aab3fab616cecb0",
			},
			{
				"0x6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0",
				"0x7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
			},
		},
		"0x6ee3440b08a9c805305449ec7f7003f27e9f7e287b83610952ec36bdc5a6bae": {
			{
				"0x1e2cd4b3588e8f6f9c4e89fb0e293bf92018c96d7a93ee367d29a284223b6ff",
				"0x71d1e9d188c784a0bde95c1d508877a0d93e9102b37213d1e13f3ebc54a7751",
			},
			{
				"0x5f750dc13ed239fa6fc43ff6e10ae9125a33bd05ec034fc3bb4dd168df3505f",
				"0x7e5",
			},
			{
				"0x48cba68d4e86764105adcdcf641ab67b581a55a4f367203647549c8bf1feea2",
				"0x362d24a3b030998ac75e838955dfee19ec5b6eceb235b9bfbeccf51b6304d0b",
			},
			{
				"0x449908c349e90f81ab13042b1e49dc251eb6e3e51092d9a40f86859f7f415b0",
				"0x6cb6104279e754967a721b52bcf5be525fdc11fa6db6ef5c3a4db832acf7804",
			},
			{
				"0x5bdaf1d47b176bfcd1114809af85a46b9c4376e87e361d86536f0288a284b65",
				"0x28dff6722aa73281b2cf84cac09950b71fa90512db294d2042119abdd9f4b87",
			},
			{
				"0x5bdaf1d47b176bfcd1114809af85a46b9c4376e87e361d86536f0288a284b66",
				"0x57a8f8a019ccab5bfc6ff86c96b1392257abb8d5d110c01d326b94247af161c",
			},
		},
		"0x31c887d82502ceb218c06ebb46198da3f7b92864a8223746bc836dda3e34b52": {
			{
				"0x5f750dc13ed239fa6fc43ff6e10ae9125a33bd05ec034fc3bb4dd168df3505f",
				"0x7c7",
			},
			{
				"0xdf28e613c065616a2e79ca72f9c1908e17b8c913972a9993da77588dc9cae9",
				"0x1432126ac23c7028200e443169c2286f99cdb5a7bf22e607bcd724efa059040",
			},
		},
		"0x31c9cdb9b00cb35cf31c05855c0ec3ecf6f7952a1ce6e3c53c3455fcd75a280": {
			{"0x5", "0x65"},
			{
				"0x5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5",
				"0x7c7",
			},
			{
				"0xcfc2e2866fd08bfb4ac73b70e0c136e326ae18fc797a2c090c8811c695577e",
				"0x5f1dd5a5aef88e0498eeca4e7b2ea0fa7110608c11531278742f0b5499af4b3",
			},
			{
				"0x5fac6815fddf6af1ca5e592359862ede14f171e1544fd9e792288164097c35d",
				"0x299e2f4b5a873e95e65eb03d31e532ea2cde43b498b50cd3161145db5542a5",
			},
			{
				"0x5fac6815fddf6af1ca5e592359862ede14f171e1544fd9e792288164097c35e",
				"0x3d6897cf23da3bf4fd35cc7a43ccaf7c5eaf8f7c5b9031ac9b09a929204175f",
			},
		},
	}

	want, err := new(felt.Felt).SetString("0x021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6")
	require.NoError(t, err)

	contractHash, err := new(felt.Felt).SetString("0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	require.NoError(t, err)

	RunOnTempTrie(251, func(state *Trie) error {
		for addr, dif := range addresses {
			RunOnTempTrie(251, func(contractState *Trie) error {
				for _, slot := range dif {
					key, err := new(felt.Felt).SetString(slot.key)
					require.NoError(t, err)

					val, err := new(felt.Felt).SetString(slot.val)
					require.NoError(t, err)

					_, err = contractState.Put(key, val)
					require.NoError(t, err)
				}
				/*
				   735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c  :  15c52969f4ae2ad48bf324e21b8c06ce8abcbc492263072a8de9c7f0bfa3c81
				   20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6  :  4532b9a656bd6074c2ddb1b884fb976eb055cd4d37e093448ce3f223864ccc4
				   6ee3440b08a9c805305449ec7f7003f27e9f7e287b83610952ec36bdc5a6bae  :  51c6b823cbf53c47ab7b34cddf1d9c0286fbb9d72ab29f2b577da0308cb1a07
				   31c887d82502ceb218c06ebb46198da3f7b92864a8223746bc836dda3e34b52  :  2eb33f71cbf096ea6b3a55ba19fb31efc31184caca6482bc89c7708c2cbb420
				   31c9cdb9b00cb35cf31c05855c0ec3ecf6f7952a1ce6e3c53c3455fcd75a280  :  6fe0662f4be66647b4508a53a08e13e7d1ffb2b19e93fa9dc991153f3a447d
				*/

				key, err := new(felt.Felt).SetString(addr)
				require.NoError(t, err)

				contractRoot, err := contractState.Root()
				require.NoError(t, err)

				fmt.Println(addr, " : ", contractRoot.String())

				val := crypto.Pedersen(contractHash, contractRoot)
				val = crypto.Pedersen(val, new(felt.Felt))
				val = crypto.Pedersen(val, new(felt.Felt))

				_, err = state.Put(key, val)
				require.NoError(t, err)

				return nil
			})
		}

		got, err := state.Root()
		require.NoError(t, err)

		assert.Equal(t, want, got)

		return nil
	})
}

func TestPutZero(t *testing.T) {
	storage := newMemStorage()
	trie := NewTrie(storage, 251, nil)
	emptyRoot, err := trie.Root()
	require.NoError(t, err)

	var roots []*felt.Felt
	var keys []*felt.Felt
	// put random 64 keys and record roots
	for i := 0; i < 64; i++ {
		key, err := new(felt.Felt).SetRandom()
		require.NoError(t, err)

		value, err := new(felt.Felt).SetRandom()
		require.NoError(t, err)

		_, err = trie.Put(key, value)
		require.NoError(t, err)

		keys = append(keys, key)
		root, err := trie.Root()
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

	// put zero in reverse order and check roots still match
	for i := 0; i < 64; i++ {
		root := roots[len(roots)-1-i]

		actual, err := trie.Root()
		require.NoError(t, err)

		assert.Equal(t, true, actual.Equal(root))

		key := keys[len(keys)-1-i]
		trie.Put(key, new(felt.Felt))
	}

	actualEmptyRoot, err := trie.Root()
	require.NoError(t, err)

	assert.Equal(t, true, actualEmptyRoot.Equal(emptyRoot))
	assert.Zero(t, len(storage.storage)) // storage should be empty
}

func TestOldData(t *testing.T) {
	RunOnTempTrie(251, func(trie *Trie) error {
		key := new(felt.Felt).SetUint64(12)
		old := new(felt.Felt)

		was, err := trie.Put(key, old)
		require.NoError(t, err)

		assert.Nil(t, was) // no change

		was, err = trie.Put(key, new(felt.Felt).SetUint64(1))
		require.NoError(t, err)

		assert.Equal(t, old, was)

		old.SetUint64(1)

		was, err = trie.Put(key, new(felt.Felt).SetUint64(2))
		require.NoError(t, err)

		assert.Equal(t, old, was)

		old.SetUint64(2)

		// put zero value to delete current key
		was, err = trie.Put(key, new(felt.Felt))
		require.NoError(t, err)

		assert.Equal(t, old, was)

		// put zero again to check old data
		was, err = trie.Put(key, new(felt.Felt))
		require.NoError(t, err)

		// there should no old data to return
		assert.Nil(t, was)

		return nil
	})
}
