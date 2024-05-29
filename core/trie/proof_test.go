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

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
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

func buildSimpleDoubleBinaryTrie(t *testing.T) *trie.Trie {
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
	return tempTrie
}

func build3KeyTrie(t *testing.T) *trie.Trie {

	// 			Starknet
	//
	//			Edge
	//			|
	//			Binary with len 249
	//		/				\
	//	Binary (250)	Edge with len 250 (?)
	//	/	\				\
	// 0x4	0x5			0x6 (edge?)

	//			 Juno
	//
	//		Node (path 249)
	//		/			\
	//  Node (binary)	0x6
	//	/	\
	// 0x4	0x5

	// Build trie
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)

	// Update trie
	key1 := new(felt.Felt).SetUint64(0)
	key2 := new(felt.Felt).SetUint64(1)
	key3 := new(felt.Felt).SetUint64(2)
	value1 := new(felt.Felt).SetUint64(4)
	value2 := new(felt.Felt).SetUint64(5)
	value3 := new(felt.Felt).SetUint64(6)

	_, err = tempTrie.Put(key1, value1)
	require.NoError(t, err)

	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)

	_, err = tempTrie.Put(key3, value3)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())
	return tempTrie
}

func TestGetProofs(t *testing.T) {
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

		proofNodes, err := trie.GetProof(new(felt.Felt).SetUint64(0), tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - simple double binary", func(t *testing.T) {
		tempTrie := buildSimpleDoubleBinaryTrie(t)

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

		proofNodes, err := trie.GetProof(new(felt.Felt).SetUint64(0), tempTrie)
		require.NoError(t, err)

		// Better inspection
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("Simple Trie - simple double binary edge", func(t *testing.T) {
		tempTrie := buildSimpleDoubleBinaryTrie(t)

		zero := trie.NewKey(249, []byte{0})
		value3 := new(felt.Felt).SetUint64(5)
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

		proofNodes, err := trie.GetProof(new(felt.Felt).SetUint64(3), tempTrie)
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

		proofNodes, err := trie.GetProof(new(felt.Felt).SetUint64(0), tempTrie)
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

		proofNodes, err := trie.GetProof(new(felt.Felt).SetUint64(0), tempTrie)
		require.NoError(t, err)

		// Better inspection
		for i, pNode := range proofNodes {
			fmt.Println(i)
			pNode.PrettyPrint()
		}
		require.Equal(t, expectedProofNodes, proofNodes)
	})
}

func TestVerifyProofs(t *testing.T) {
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
		val1 := new(felt.Felt).SetUint64(2)
		assert.True(t, trie.VerifyProof(root, new(felt.Felt).SetUint64(0), val1, expectedProofNodes, crypto.Pedersen))
	})

	// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2167
	t.Run("Simple double binary trie", func(t *testing.T) {
		tempTrie := buildSimpleDoubleBinaryTrie(t)
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
		val1 := new(felt.Felt).SetUint64(2)
		assert.True(t, trie.VerifyProof(root, new(felt.Felt).SetUint64(0), val1, expectedProofNodes, crypto.Pedersen))
	})

	t.Run("Simple binary trie - fail missing key", func(t *testing.T) {
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
		key1 := utils.HexToFelt(t, "0x4")   // Non-existant key
		val1 := new(felt.Felt).SetUint64(2) // key=0 has val=2
		assert.False(t, trie.VerifyProof(root, key1, val1, expectedProofNodes, crypto.Pedersen))
	})
}

func TestProofToPath(t *testing.T) {

	t.Run("Simple binary trie proof to path", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)
		zero := trie.NewKey(250, []byte{0})
		proofNodes := []trie.ProofNode{
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
		leafKey := new(felt.Felt).SetUint64(0)
		sns, err := trie.ProofToPath(proofNodes, leafKey, crypto.Pedersen)
		require.NoError(t, err)

		rootKey := tempTrie.RootKey()
		rootNodes, err := tempTrie.GetNodeFromKey(rootKey)
		require.NoError(t, err)

		require.Equal(t, 1, len(sns))
		require.Equal(t, rootKey.Len(), sns[0].Key().Len())
		require.Equal(t, rootNodes.Left, sns[0].Node().Left)
		require.NotEqual(t, rootNodes.Right, sns[0].Node().Right)
	})

	t.Run("Simple doulbe binary trie proof to path", func(t *testing.T) {
		tempTrie := buildSimpleBinaryRootTrie(t)

		key1Bytes := new(felt.Felt).SetUint64(0).Bytes()
		path1 := trie.NewKey(250, key1Bytes[:])
		proofNodes := []trie.ProofNode{
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

		leafKey := new(felt.Felt).SetUint64(0)
		sns, err := trie.ProofToPath(proofNodes, leafKey, crypto.Pedersen)
		require.NoError(t, err)

		rootKey := tempTrie.RootKey()

		rootNodes, err := tempTrie.GetNodeFromKey(rootKey)
		require.NoError(t, err)
		fmt.Println(rootNodes.Value.String())
		require.Equal(t, 1, len(sns))
		require.Equal(t, rootKey.Len(), sns[0].Key().Len())
		require.Equal(t, rootNodes.Left, sns[0].Node().Left)
		require.NotEqual(t, rootNodes.Right, sns[0].Node().Right)
	})

	t.Run("boundary proofs wth three key trie", func(t *testing.T) {
		tri := build3KeyTrie(t)
		rootKey := tri.RootKey()
		rootNode, err := tri.GetNodeFromKey(rootKey)
		require.NoError(t, err)
		fmt.Println(rootNode)

		key1 := new(felt.Felt).SetUint64(0)
		key3 := new(felt.Felt).SetUint64(2)
		bProofs, err := trie.GetBoundaryProofs(key1, key3, tri)
		require.NoError(t, err)

		trie.PrettyPrintProofPath(bProofs[0], key1)
		trie.PrettyPrintProofPath(bProofs[1], key3)

		expectedRoot := trie.NewStorageNode(rootKey, &trie.Node{Value: rootNode.Value, Left: rootNode.Left})

		leftProofPath, err := trie.ProofToPath(bProofs[0], key1, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, expectedRoot.Key(), leftProofPath[0].Key())
		require.Equal(t, expectedRoot.Node().Left, leftProofPath[0].Node().Left)
		require.Equal(t, expectedRoot.Node().Right, leftProofPath[0].Node().Right)

		rightProofPath, err := trie.ProofToPath(bProofs[1], key3, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, expectedRoot.Key(), rightProofPath[0].Key())
		require.Equal(t, expectedRoot.Node().Left, rightProofPath[0].Node().Left)
		require.Equal(t, expectedRoot.Node().Right, rightProofPath[0].Node().Right)

		// Todo: check second nodes
	})
}
func TestBuildTrie(t *testing.T) {

	t.Run("Simple binary trie proof to path", func(t *testing.T) {
		tri := build3KeyTrie(t)
		rootKey := tri.RootKey()
		nodes, err := tri.GetNodeFromKey(rootKey)
		require.NoError(t, err)
		fmt.Println(nodes)

		key1 := new(felt.Felt).SetUint64(0)
		key3 := new(felt.Felt).SetUint64(2)
		bProofs, err := trie.GetBoundaryProofs(key1, key3, tri)
		require.NoError(t, err)

		leftProof, err := trie.ProofToPath(bProofs[0], key1, crypto.Pedersen)
		require.NoError(t, err)

		require.Equal(t, leftProof[0].Key().String(), rootKey.String())
		require.NotNil(t, leftProof[0].Node().Left)
		require.Equal(t, leftProof[0].Node().Left.Len(), 250)

		rightProof, err := trie.ProofToPath(bProofs[1], key3, crypto.Pedersen)
		require.NoError(t, err)

		require.Equal(t, rightProof[0].Key().String(), rootKey.String())
		require.NotNil(t, rightProof[0].Node().Right)
		require.Equal(t, rightProof[0].Node().Right.Len(), 251)

		keys := []*felt.Felt{new(felt.Felt).SetUint64(1)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(5)}
		reconstructedTrie, err := trie.BuildTrie(leftProof, rightProof, keys, values)
		require.NoError(t, err)

		reconstructedRoot, err := reconstructedTrie.Root()
		require.NoError(t, err)
		expectedRoot, err := tri.Root()
		require.NoError(t, err)
		require.Equal(t, expectedRoot, reconstructedRoot)
	})
}
