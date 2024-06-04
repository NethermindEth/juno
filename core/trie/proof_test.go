package trie_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildSimpleTrie(t *testing.T) *trie.Trie {
	//   (250, 0, x1)
	//        |
	//     (0,0,x1)
	//      /    \
	//     (2)  (3)
	// Build trie
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{1}), 251)
	require.NoError(t, err)

	// Update trie
	key1 := new(felt.Felt).SetUint64(0)
	key2 := new(felt.Felt).SetUint64(1)
	value1 := new(felt.Felt).SetUint64(2)
	value2 := new(felt.Felt).SetUint64(3)

	_, err = tempTrie.Put(key1, value1)
	require.NoError(t, err)

	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())
	return tempTrie
}

func buildSimpleBinaryRootTrie(t *testing.T) *trie.Trie {
	//           (0, 0, x)
	//    /                    \
	// (250, 0, cc)     (250, 11111.., dd)
	//    |                     |
	//   (cc)                  (dd)
	// Build trie
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{1}), 251)
	require.NoError(t, err)

	key1 := new(felt.Felt).SetUint64(0)
	key2 := utils.HexToFelt(t, "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	value1 := utils.HexToFelt(t, "0xcc")
	value2 := utils.HexToFelt(t, "0xdd")

	_, err = tempTrie.Put(key1, value1)
	require.NoError(t, err)

	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())
	return tempTrie
}

func buildSimpleDoubleBinaryTrie(t *testing.T) (*trie.Trie, []trie.ProofNode) {
	//           (249,0,x3)         // Edge
	//               |
	//           (0, 0, x3)         // Binary
	//         /            \
	//     (0,0,x1) // B  (1, 1, 5) // Edge leaf
	//      /    \             |
	//     (2)  (3)           (5)

	// Build trie
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{1}), 251)
	require.NoError(t, err)

	// Update trie
	key1 := new(felt.Felt).SetUint64(0)
	key2 := new(felt.Felt).SetUint64(1)
	key3 := new(felt.Felt).SetUint64(3)
	value1 := new(felt.Felt).SetUint64(2)
	value2 := new(felt.Felt).SetUint64(3)
	value3 := new(felt.Felt).SetUint64(5)

	_, err = tempTrie.Put(key1, value1)
	require.NoError(t, err)

	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)

	_, err = tempTrie.Put(key3, value3)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())

	zero := trie.NewKey(249, []byte{0})
	key3Bytes := new(felt.Felt).SetUint64(1).Bytes()
	path3 := trie.NewKey(1, key3Bytes[:])
	expectedProofNodes := []trie.ProofNode{
		{
			Edge: &trie.Edge{
				Path:  &zero,
				Child: utils.HexToFelt(t, "0x055C81F6A791FD06FC2E2CCAD922397EC76C3E35F2E06C0C0D43D551005A8DEA"),
			},
		},
		{
			Binary: &trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
				RightHash: utils.HexToFelt(t, "0x07C5BC1CC68B7BC8CA2F632DE98297E6DA9594FA23EDE872DD2ABEAFDE353B43"),
			},
		},
		{
			Edge: &trie.Edge{
				Path:  &path3,
				Child: value3,
			},
		},
	}

	return tempTrie, expectedProofNodes
}

func TestGetProof(t *testing.T) {
	t.Run("Simple Trie - simple binary", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)

		zero := trie.NewKey(250, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero,
					Child: utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
				},
			},
			{
				Binary: &trie.Binary{
					LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
					RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
				},
			},
		}
		leafFelt := new(felt.Felt).SetUint64(0).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - simple double binary", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)

		expectedProofNodes[2] = trie.ProofNode{
			Binary: &trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
				RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
			},
		}

		leafFelt := new(felt.Felt).SetUint64(0).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - simple double binary edge", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)
		leafFelt := new(felt.Felt).SetUint64(3).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - simple binary root", func(t *testing.T) {
		tempTrie := buildSimpleBinaryRootTrie(t)

		key1Bytes := new(felt.Felt).SetUint64(0).Bytes()
		path1 := trie.NewKey(250, key1Bytes[:])
		expectedProofNodes := []trie.ProofNode{
			{
				Binary: &trie.Binary{
					LeftHash:  utils.HexToFelt(t, "0x06E08BF82793229338CE60B65D1845F836C8E2FBFE2BC59FF24AEDBD8BA219C4"),
					RightHash: utils.HexToFelt(t, "0x04F9B8E66212FB528C0C1BD02F43309C53B895AA7D9DC91180001BDD28A588FA"),
				},
			},
			{
				Edge: &trie.Edge{
					Path:  &path1,
					Child: utils.HexToFelt(t, "0xcc"),
				},
			},
		}
		leafFelt := new(felt.Felt).SetUint64(0).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])

		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - left-right edge", func(t *testing.T) {
		//  (251,0xff,0xaa)
		//     /
		//     \
		//   (0xaa)
		memdb := pebble.NewMemTest(t)
		txn, err := memdb.NewTransaction(true)
		require.NoError(t, err)

		tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{1}), 251)
		require.NoError(t, err)

		key1 := utils.HexToFelt(t, "0xff")
		value1 := utils.HexToFelt(t, "0xaa")

		_, err = tempTrie.Put(key1, value1)
		require.NoError(t, err)

		require.NoError(t, tempTrie.Commit())

		key1Bytes := key1.Bytes()
		path1 := trie.NewKey(251, key1Bytes[:])

		child := utils.HexToFelt(t, "0x00000000000000000000000000000000000000000000000000000000000000AA")
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &path1,
					Child: child,
				},
			},
		}
		leafFelt := new(felt.Felt).SetUint64(0).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for i, pNode := range proofNodes {
			fmt.Println(i)
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - proof for non-set key", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)

		leafFelt := new(felt.Felt).SetUint64(123).Bytes() // The (root) edge node would have a shorter len if this key was set
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes[0:2], proofNodes)
	})

	t.Run("Simple Trie - proof for inner key", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)

		innerFelt := new(felt.Felt).SetUint64(2).Bytes()
		innerKey := trie.NewKey(123, innerFelt[:]) // The (root) edge node has len 249 which shows this doesn't exist
		proofNodes, err := trie.GetProof(&innerKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes[0:2], proofNodes)
	})

	t.Run("Simple Trie - proof for non-set key, with leafs set to right and left", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)

		leafFelt := new(felt.Felt).SetUint64(2).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})
}

func TestVerifyProof(t *testing.T) {
	// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2137
	t.Run("Simple binary trie", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)
		zero := trie.NewKey(250, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero,
					Child: utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
				},
			},
			{
				Binary: &trie.Binary{
					LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
					RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
				},
			},
		}

		root, err := tempTrie.Root()
		require.NoError(t, err)
		key1Bytes := new(felt.Felt).SetUint64(0).Bytes()
		key1 := trie.NewKey(251, key1Bytes[:])
		val1 := new(felt.Felt).SetUint64(2)
		assert.True(t, trie.VerifyProof(root, &key1, val1, expectedProofNodes, crypto.Pedersen))
	})

	// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2167
	t.Run("Simple double binary trie", func(t *testing.T) {
		tempTrie, _ := buildSimpleDoubleBinaryTrie(t)
		zero := trie.NewKey(249, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero,
					Child: utils.HexToFelt(t, "0x055C81F6A791FD06FC2E2CCAD922397EC76C3E35F2E06C0C0D43D551005A8DEA"),
				},
			},
			{
				Binary: &trie.Binary{
					LeftHash:  utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
					RightHash: utils.HexToFelt(t, "0x07C5BC1CC68B7BC8CA2F632DE98297E6DA9594FA23EDE872DD2ABEAFDE353B43"),
				},
			},
			{
				Binary: &trie.Binary{
					LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
					RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
				},
			},
		}

		root, err := tempTrie.Root()
		require.NoError(t, err)
		key1Bytes := new(felt.Felt).SetUint64(0).Bytes()
		key1 := trie.NewKey(251, key1Bytes[:])
		val1 := new(felt.Felt).SetUint64(2)
		require.True(t, trie.VerifyProof(root, &key1, val1, expectedProofNodes, crypto.Pedersen))
	})

	t.Run("non existent key - less than root edge", func(t *testing.T) {
		tempTrie, _ := buildSimpleDoubleBinaryTrie(t)

		nonExistentKey := trie.NewKey(123, []byte{0}) // Diverges before the root node (len root node = 249)
		nonExistentKeyValue := new(felt.Felt).SetUint64(2)
		proofNodes, err := trie.GetProof(&nonExistentKey, tempTrie)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		require.False(t, trie.VerifyProof(root, &nonExistentKey, nonExistentKeyValue, proofNodes, crypto.Pedersen))
	})

	t.Run("non existent leaf key", func(t *testing.T) {
		tempTrie, _ := buildSimpleDoubleBinaryTrie(t)

		nonExistentKeyByte := new(felt.Felt).SetUint64(2).Bytes() // Key not set
		nonExistentKey := trie.NewKey(251, nonExistentKeyByte[:])
		nonExistentKeyValue := new(felt.Felt).SetUint64(2)
		proofNodes, err := trie.GetProof(&nonExistentKey, tempTrie)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		require.False(t, trie.VerifyProof(root, &nonExistentKey, nonExistentKeyValue, proofNodes, crypto.Pedersen))
	})
}
