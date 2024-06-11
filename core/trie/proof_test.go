package trie_test

import (
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
	//   (250, 0, x1)		edge
	//        |
	//     (0,0,x1)			binary
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

	_, err = tempTrie.Put(key3, value3)
	require.NoError(t, err)
	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())
	return tempTrie
}

func build4KeyTrie(t *testing.T) *trie.Trie {
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
	key5 := new(felt.Felt).SetUint64(4)
	value1 := new(felt.Felt).SetUint64(4)
	value2 := new(felt.Felt).SetUint64(5)
	value3 := new(felt.Felt).SetUint64(6)
	value5 := new(felt.Felt).SetUint64(7)

	_, err = tempTrie.Put(key1, value1)
	require.NoError(t, err)

	_, err = tempTrie.Put(key3, value3)
	require.NoError(t, err)
	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)
	_, err = tempTrie.Put(key5, value5)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())

	return tempTrie
}

func TestGetProof(t *testing.T) {
	t.Run("GP Simple Trie - simple binary", func(t *testing.T) {
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
		// for _, pNode := range proofNodes {
		// 	pNode.PrettyPrint()
		// }
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("GP Simple Trie - simple double binary", func(t *testing.T) {
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
		// for _, pNode := range proofNodes {
		// 	pNode.PrettyPrint()
		// }
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("GP Simple Trie - simple double binary edge", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)
		leafFelt := new(felt.Felt).SetUint64(3).Bytes()
		leafKey := trie.NewKey(251, leafFelt[:])
		proofNodes, err := trie.GetProof(&leafKey, tempTrie)
		require.NoError(t, err)

		// Better inspection
		// for _, pNode := range proofNodes {
		// 	pNode.PrettyPrint()
		// }
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("GP Simple Trie - simple binary root", func(t *testing.T) {
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
		// for _, pNode := range proofNodes {
		// 	pNode.PrettyPrint()
		// }
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("GP Simple Trie - left-right edge", func(t *testing.T) {
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
		// for _, pNode := range proofNodes {
		// 	pNode.PrettyPrint()
		// }
		require.Equal(t, expectedProofNodes, proofNodes)
	})

	t.Run("GP Simple Trie - proof for non-set key", func(t *testing.T) {
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

	t.Run("GP Simple Trie - proof for inner key", func(t *testing.T) {
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

	t.Run("GP Simple Trie - proof for non-set key, with leafs set to right and left", func(t *testing.T) {
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
	t.Run("VP Simple binary trie", func(t *testing.T) {
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

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		leafkey := trie.NewKey(251, zeroFeltBytes[:])
		assert.True(t, trie.VerifyProof(root, &leafkey, val1, expectedProofNodes, crypto.Pedersen))
	})

	// https://github.com/eqlabs/pathfinder/blob/main/crates/merkle-tree/src/tree.rs#L2167
	t.Run("VP Simple double binary trie", func(t *testing.T) {
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
		val1 := new(felt.Felt).SetUint64(2)
		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		leafkey := trie.NewKey(251, zeroFeltBytes[:])
		assert.True(t, trie.VerifyProof(root, &leafkey, val1, expectedProofNodes, crypto.Pedersen))
	})

	t.Run("VP  three key trie", func(t *testing.T) {
		tempTrie := build3KeyTrie(t)
		zero := trie.NewKey(249, []byte{0})
		felt2 := new(felt.Felt).SetUint64(0).Bytes()
		lastPath := trie.NewKey(1, felt2[:])
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero,
					Child: utils.HexToFelt(t, "0x0768DEB8D0795D80AAAC2E5E326141F33044759F97A1BF092D8EB9C4E4BE9234"),
				},
			},
			{
				Binary: &trie.Binary{
					LeftHash:  utils.HexToFelt(t, "0x057166F9476D0A2D6875124251841EB85A9AE37462FAE3CBF7304BCD593938E7"),
					RightHash: utils.HexToFelt(t, "0x060FBDE29F96F706498EFD132DC7F312A4C99A9AE051BF152C2AF2B3CAF31E5B"),
				},
			},
			{
				Edge: &trie.Edge{
					Path:  &lastPath,
					Child: utils.HexToFelt(t, "0x6"),
				},
			},
		}

		root, err := tempTrie.Root()
		require.NoError(t, err)
		val6 := new(felt.Felt).SetUint64(6)

		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
		leafkey := trie.NewKey(251, twoFeltBytes[:])
		gotProof, err := trie.GetProof(&leafkey, tempTrie)
		require.NoError(t, err)
		require.Equal(t, expectedProofNodes, gotProof)

		assert.True(t, trie.VerifyProof(root, &leafkey, val6, expectedProofNodes, crypto.Pedersen))
	})

	t.Run("VP  non existent key - less than root edge", func(t *testing.T) {
		tempTrie, _ := buildSimpleDoubleBinaryTrie(t)

		nonExistentKey := trie.NewKey(123, []byte{0}) // Diverges before the root node (len root node = 249)
		nonExistentKeyValue := new(felt.Felt).SetUint64(2)
		proofNodes, err := trie.GetProof(&nonExistentKey, tempTrie)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		require.False(t, trie.VerifyProof(root, &nonExistentKey, nonExistentKeyValue, proofNodes, crypto.Pedersen))
	})

	t.Run("VP  non existent leaf key", func(t *testing.T) {
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

func TestProofToPath(t *testing.T) {
	t.Run("PTP Proof To Path Simple binary trie proof to path", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)
		zeroFeltByte := new(felt.Felt).Bytes()
		zero := trie.NewKey(250, zeroFeltByte[:])
		leafValue := utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002")
		siblingValue := utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003")
		proofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero,
					Child: utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
				},
			},
			{
				Binary: &trie.Binary{
					LeftHash:  leafValue,
					RightHash: siblingValue,
				},
			},
		}

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		leafkey := trie.NewKey(251, zeroFeltBytes[:])
		sns, err := trie.ProofToPath(proofNodes, &leafkey, crypto.Pedersen)
		require.NoError(t, err)

		rootKey := tempTrie.RootKey()

		require.Equal(t, 1, len(sns))
		require.Equal(t, rootKey.Len(), sns[0].Key().Len())
		require.Equal(t, leafValue.String(), sns[0].Node().LeftHash.String())
		require.Equal(t, siblingValue.String(), sns[0].Node().RightHash.String())
	})

	t.Run("PTP Simple double binary trie proof to path", func(t *testing.T) {
		tempTrie := buildSimpleBinaryRootTrie(t)

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		leafkey := trie.NewKey(251, zeroFeltBytes[:])
		path1 := trie.NewKey(250, zeroFeltBytes[:])
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

		leafValue := utils.HexToFelt(t, "0xcc")
		siblingValue := utils.HexToFelt(t, "0xdd")

		sns, err := trie.ProofToPath(proofNodes, &leafkey, crypto.Pedersen)
		require.NoError(t, err)

		rootKey := tempTrie.RootKey()

		require.Equal(t, 1, len(sns))
		require.Equal(t, rootKey.Len(), sns[0].Key().Len())
		require.Equal(t, leafValue.String(), sns[0].Node().LeftHash.String())
		require.NotEqual(t, siblingValue.String(), sns[0].Node().RightHash.String())
	})

	t.Run("PTP  boundary proofs with three key trie", func(t *testing.T) {
		tri := build3KeyTrie(t)
		rootKey := tri.RootKey()
		rootNode, err := tri.GetNodeFromKey(rootKey)
		require.NoError(t, err)

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])
		zeroLeafValue := new(felt.Felt).SetUint64(4)
		oneLeafValue := new(felt.Felt).SetUint64(5)
		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
		twoLeafkey := trie.NewKey(251, twoFeltBytes[:])
		twoLeafValue := new(felt.Felt).SetUint64(6)
		bProofs, err := trie.GetBoundaryProofs(&zeroLeafkey, &twoLeafkey, tri)
		require.NoError(t, err)

		// Test 1
		leftProofPath, err := trie.ProofToPath(bProofs[0], &zeroLeafkey, crypto.Pedersen)
		require.Equal(t, 2, len(leftProofPath))
		// require.NoError(t, err)
		// left, err := tri.GetNodeFromKey(rootNode.Left)
		// require.NoError(t, err)
		// right, err := tri.GetNodeFromKey(rootNode.Right)
		require.NoError(t, err)
		require.Equal(t, rootKey, leftProofPath[0].Key())
		// require.Equal(t, left.Hash(rootNode.Left, crypto.Pedersen).String(), leftProofPath[0].Node().LeftHash.String()) // Todo
		// require.Equal(t, right.Hash(rootNode.Right, crypto.Pedersen).String(), leftProofPath[0].Node().RightHash.String()) // Todo
		require.Equal(t, rootNode.Left, leftProofPath[1].Key())
		require.Equal(t, zeroLeafValue.String(), leftProofPath[1].Node().LeftHash.String())
		require.Equal(t, oneLeafValue.String(), leftProofPath[1].Node().RightHash.String())

		// Test 2
		rightProofPath, err := trie.ProofToPath(bProofs[1], &twoLeafkey, crypto.Pedersen)
		require.Equal(t, 1, len(rightProofPath))
		require.NoError(t, err)
		require.Equal(t, rootKey, rightProofPath[0].Key())
		require.Equal(t, rootNode.Right, rightProofPath[0].Node().Right)
		require.Equal(t, twoLeafValue.String(), rightProofPath[0].Node().RightHash.String())
	})
}

func TestBuildTrie(t *testing.T) {
	t.Run("Simple binary trie proof to path", func(t *testing.T) {
		compareLeftRight := func(t *testing.T, want, got *trie.Node) {
			require.Equal(t, want.Left, got.Left, "left fail")
			require.Equal(t, want.Right, got.Right, "right fail")
		}

		//		Node (edge path 249)
		//		/			\
		//  Node (binary)	0x6 (leaf)
		//	/	\
		// 0x4	0x5 (leaf, leaf)

		tri := build3KeyTrie(t)
		rootKey := tri.RootKey()
		rootCommitment, err := tri.Root()
		require.NoError(t, err)
		rootNode, err := tri.GetNodeFromKey(rootKey)
		require.NoError(t, err)
		leftNode, err := tri.GetNodeFromKey(rootNode.Left)
		require.NoError(t, err)
		rightNode, err := tri.GetNodeFromKey(rootNode.Right)
		require.NoError(t, err)
		leftleftNode, err := tri.GetNodeFromKey(leftNode.Left)
		require.NoError(t, err)
		leftrightNode, err := tri.GetNodeFromKey(leftNode.Right)
		require.NoError(t, err)

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])
		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
		twoLeafkey := trie.NewKey(251, twoFeltBytes[:])
		bProofs, err := trie.GetBoundaryProofs(&zeroLeafkey, &twoLeafkey, tri)
		require.NoError(t, err)

		leftProof, err := trie.ProofToPath(bProofs[0], &zeroLeafkey, crypto.Pedersen)
		require.NoError(t, err)

		rightProof, err := trie.ProofToPath(bProofs[1], &twoLeafkey, crypto.Pedersen)
		require.NoError(t, err)

		keys := []*felt.Felt{new(felt.Felt).SetUint64(1)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(5)}
		builtTrie, err := trie.BuildTrie(leftProof, rightProof, keys, values)
		require.NoError(t, err)

		builtRootKey := builtTrie.RootKey()
		builtRootNode, err := builtTrie.GetNodeFromKey(builtRootKey)
		require.NoError(t, err)
		builtLeftNode, err := builtTrie.GetNodeFromKey(builtRootNode.Left)
		require.NoError(t, err)
		builtRightNode, err := builtTrie.GetNodeFromKey(builtRootNode.Right)
		require.NoError(t, err)
		builtLeftLeftNode, err := builtTrie.GetNodeFromKey(builtLeftNode.Left)
		require.NoError(t, err)
		builtLeftRightNode, err := builtTrie.GetNodeFromKey(builtLeftNode.Right)
		require.NoError(t, err)

		// Assert the structure is correct
		require.Equal(t, rootKey, builtRootKey)
		compareLeftRight(t, rootNode, builtRootNode)
		compareLeftRight(t, leftNode, builtLeftNode)
		compareLeftRight(t, rightNode, builtRightNode)
		compareLeftRight(t, leftleftNode, builtLeftLeftNode)
		compareLeftRight(t, leftrightNode, builtLeftRightNode)

		// Assert the leaf nodes have the correct values
		require.Equal(t, leftleftNode.Value.String(), builtLeftLeftNode.Value.String(), "should be 0x4")
		require.Equal(t, leftrightNode.Value.String(), builtLeftRightNode.Value.String(), "should be 0x5")
		require.Equal(t, rightNode.Value.String(), builtRightNode.Value.String(), "should be 0x6")

		// Given the above two asserts pass, we should be able to reconstruct the correct commitment
		reconstructedRootCommitment, err := builtTrie.Root()
		require.NoError(t, err)
		require.Equal(t, rootCommitment.String(), reconstructedRootCommitment.String())
	})
}

func TestVerifyRangeProof(t *testing.T) {
	t.Run("two proofs, single key trie", func(t *testing.T) {
		//		Node (edge path 249)
		//		/			\
		//  Node (binary)	0x6 (leaf)
		//	/	\
		// 0x4	0x5 (leaf, leaf)

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])
		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
		twoLeafkey := trie.NewKey(251, twoFeltBytes[:])

		tri := build3KeyTrie(t)
		keys := []*felt.Felt{new(felt.Felt).SetUint64(1)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(5)}
		proofKeys := [2]*trie.Key{&zeroLeafkey, &twoLeafkey}
		proofValues := [2]*felt.Felt{new(felt.Felt).SetUint64(4), new(felt.Felt).SetUint64(6)}
		rootCommitment, err := tri.Root()
		require.NoError(t, err)
		proofs, err := trie.GetBoundaryProofs(proofKeys[0], proofKeys[1], tri)
		require.NoError(t, err)
		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
		require.NoError(t, err)
		require.True(t, verif)
	})

	t.Run("all keys provided, no proofs needed", func(t *testing.T) {
		//		Node (edge path 249)
		//		/			\
		//  Node (binary)	0x6 (leaf)
		//	/	\
		// 0x4	0x5 (leaf, leaf)
		tri := build3KeyTrie(t)
		keys := []*felt.Felt{new(felt.Felt).SetUint64(0), new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(4), new(felt.Felt).SetUint64(5), new(felt.Felt).SetUint64(6)}
		proofKeys := [2]*trie.Key{}
		proofValues := [2]*felt.Felt{}
		proofs := [2][]trie.ProofNode{}
		rootCommitment, err := tri.Root()
		require.NoError(t, err)
		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
		require.NoError(t, err)
		require.True(t, verif)
	})

	t.Run("left proof, all right keys", func(t *testing.T) {
		//		Node (edge path 249)
		//		/			\
		//  Node (binary)	0x6 (leaf)
		//	/	\
		// 0x4	0x5 (leaf, leaf)

		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])

		tri := build3KeyTrie(t)
		keys := []*felt.Felt{new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(5), new(felt.Felt).SetUint64(6)}
		proofKeys := [2]*trie.Key{&zeroLeafkey}
		proofValues := [2]*felt.Felt{new(felt.Felt).SetUint64(4)}
		leftProof, err := trie.GetProof(proofKeys[0], tri)
		require.NoError(t, err)
		proofs := [2][]trie.ProofNode{leftProof}
		rootCommitment, err := tri.Root()
		require.NoError(t, err)
		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
		require.NoError(t, err)
		require.True(t, verif)
	})

	t.Run("right proof, all left keys", func(t *testing.T) {
		//		Node (edge path 249)
		//		/			\
		//  Node (binary)	0x6 (leaf)
		//	/	\
		// 0x4	0x5 (leaf, leaf)
		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
		twoLeafkey := trie.NewKey(251, twoFeltBytes[:])

		tri := build3KeyTrie(t)
		keys := []*felt.Felt{new(felt.Felt).SetUint64(0), new(felt.Felt).SetUint64(1)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(4), new(felt.Felt).SetUint64(5)}
		proofKeys := [2]*trie.Key{nil, &twoLeafkey}
		proofValues := [2]*felt.Felt{nil, new(felt.Felt).SetUint64(6)}
		rightProof, err := trie.GetProof(proofKeys[1], tri)
		require.NoError(t, err)
		proofs := [2][]trie.ProofNode{nil, rightProof}
		rootCommitment, err := tri.Root()
		require.NoError(t, err)
		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
		require.NoError(t, err)
		require.True(t, verif)
	})

	t.Run("left proof, all inner keys, right proof with non-set key", func(t *testing.T) {
		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])

		threeFeltBytes := new(felt.Felt).SetUint64(3).Bytes()
		threeLeafkey := trie.NewKey(251, threeFeltBytes[:])

		tri := build4KeyTrie(t)
		keys := []*felt.Felt{new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2)}
		values := []*felt.Felt{new(felt.Felt).SetUint64(5), new(felt.Felt).SetUint64(6)}
		proofKeys := [2]*trie.Key{&zeroLeafkey, &threeLeafkey}
		proofValues := [2]*felt.Felt{new(felt.Felt).SetUint64(4), nil}
		leftProof, err := trie.GetProof(proofKeys[0], tri)
		require.NoError(t, err)
		rightProof, err := trie.GetProof(proofKeys[1], tri)
		require.NoError(t, err)

		proofs := [2][]trie.ProofNode{leftProof, rightProof}
		rootCommitment, err := tri.Root()
		require.NoError(t, err)

		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
		require.NoError(t, err)
		require.True(t, verif)
	})
}
