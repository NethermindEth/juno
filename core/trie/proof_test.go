package trie_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
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
		&trie.Edge{
			Path:  &zero,
			Child: utils.HexToFelt(t, "0x055C81F6A791FD06FC2E2CCAD922397EC76C3E35F2E06C0C0D43D551005A8DEA"),
		},
		&trie.Binary{
			LeftHash:  utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
			RightHash: utils.HexToFelt(t, "0x07C5BC1CC68B7BC8CA2F632DE98297E6DA9594FA23EDE872DD2ABEAFDE353B43"),
		},
		&trie.Edge{
			Path:  &path3,
			Child: value3,
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

func TestProve(t *testing.T) {
	t.Parallel()

	t.Run("simple binary", func(t *testing.T) {
		t.Parallel()

		tempTrie := buildSimpleTrie(t)

		zero := trie.NewKey(250, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			&trie.Edge{
				Path:  &zero,
				Child: utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
			},
			&trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
				RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
			},
		}

		leafFelt := new(felt.Felt).SetUint64(1)
		leafKey := tempTrie.FeltToKey(leafFelt)
		proofSet := trie.NewProofSet()
		err := tempTrie.Prove(leafFelt, proofSet)
		require.NoError(t, err)

		// Check that all expected nodes are in proof set
		require.Equal(t, len(expectedProofNodes), proofSet.Size())

		rootHash, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(rootHash, &leafKey, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, new(felt.Felt).SetUint64(3), val)
	})

	t.Run("simple double binary", func(t *testing.T) {
		t.Parallel()

		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)

		expectedProofNodes[2] = &trie.Binary{
			LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
			RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
		}

		leafFelt := new(felt.Felt).SetUint64(0)
		leafKey := tempTrie.FeltToKey(leafFelt)
		proofSet := trie.NewProofSet()
		err := tempTrie.Prove(leafFelt, proofSet)
		require.NoError(t, err)

		// Check that all expected nodes are in proof set
		require.Equal(t, len(expectedProofNodes), proofSet.Size())

		rootHash, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(rootHash, &leafKey, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, new(felt.Felt).SetUint64(2), val)
	})

	t.Run("simple double binary edge", func(t *testing.T) {
		t.Parallel()

		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)
		leafFelt := new(felt.Felt).SetUint64(3)
		leafKey := tempTrie.FeltToKey(leafFelt)
		proofSet := trie.NewProofSet()
		err := tempTrie.Prove(leafFelt, proofSet)
		require.NoError(t, err)

		require.Equal(t, len(expectedProofNodes), proofSet.Size())

		rootHash, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(rootHash, &leafKey, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, new(felt.Felt).SetUint64(5), val)
	})

	t.Run("simple binary root", func(t *testing.T) {
		t.Parallel()

		tempTrie := buildSimpleBinaryRootTrie(t)

		key1Bytes := new(felt.Felt).SetUint64(0).Bytes()
		path1 := trie.NewKey(250, key1Bytes[:])
		expectedProofNodes := []trie.ProofNode{
			&trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x06E08BF82793229338CE60B65D1845F836C8E2FBFE2BC59FF24AEDBD8BA219C4"),
				RightHash: utils.HexToFelt(t, "0x04F9B8E66212FB528C0C1BD02F43309C53B895AA7D9DC91180001BDD28A588FA"),
			},
			&trie.Edge{
				Path:  &path1,
				Child: utils.HexToFelt(t, "0xcc"),
			},
		}
		leafFelt := new(felt.Felt).SetUint64(0)
		leafKey := tempTrie.FeltToKey(leafFelt)
		proofSet := trie.NewProofSet()
		err := tempTrie.Prove(leafFelt, proofSet)
		require.NoError(t, err)

		require.Equal(t, len(expectedProofNodes), proofSet.Size())

		rootHash, err := tempTrie.Root()
		require.NoError(t, err)

		expected := utils.HexToFelt(t, "0xcc")
		val, err := trie.VerifyProof(rootHash, &leafKey, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, expected, val)
	})

	t.Run("left-right edge", func(t *testing.T) {
		t.Parallel()

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

		expectedProofNodes := []trie.ProofNode{
			&trie.Edge{
				Path:  &path1,
				Child: value1,
			},
		}

		proofSet := trie.NewProofSet()
		err = tempTrie.Prove(key1, proofSet)
		require.NoError(t, err)

		require.Equal(t, len(expectedProofNodes), proofSet.Size())

		rootHash, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(rootHash, &path1, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, value1, val)
	})

	t.Run("three key trie", func(t *testing.T) {
		t.Parallel()

		tempTrie := build3KeyTrie(t)
		zero := trie.NewKey(249, []byte{0})
		felt2 := new(felt.Felt).SetUint64(0).Bytes()
		lastPath := trie.NewKey(1, felt2[:])
		expectedProofNodes := []trie.ProofNode{
			&trie.Edge{
				Path:  &zero,
				Child: utils.HexToFelt(t, "0x0768DEB8D0795D80AAAC2E5E326141F33044759F97A1BF092D8EB9C4E4BE9234"),
			},
			&trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x057166F9476D0A2D6875124251841EB85A9AE37462FAE3CBF7304BCD593938E7"),
				RightHash: utils.HexToFelt(t, "0x060FBDE29F96F706498EFD132DC7F312A4C99A9AE051BF152C2AF2B3CAF31E5B"),
			},
			&trie.Edge{
				Path:  &lastPath,
				Child: utils.HexToFelt(t, "0x6"),
			},
		}

		root, err := tempTrie.Root()
		require.NoError(t, err)
		val6 := new(felt.Felt).SetUint64(6)

		twoFelt := new(felt.Felt).SetUint64(2)
		leafKey := tempTrie.FeltToKey(twoFelt)

		proofSet := trie.NewProofSet()
		err = tempTrie.Prove(twoFelt, proofSet)
		require.NoError(t, err)

		require.Equal(t, len(expectedProofNodes), proofSet.Size())

		val, err := trie.VerifyProof(root, &leafKey, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, val6, val)
	})

	t.Run("non existent key - less than root edge", func(t *testing.T) {
		t.Parallel()

		tempTrie, _ := buildSimpleDoubleBinaryTrie(t)

		nonExistentFelt := new(felt.Felt).SetUint64(123)
		nonExistentKey := tempTrie.FeltToKey(nonExistentFelt)
		proofSet := trie.NewProofSet()
		err := tempTrie.Prove(nonExistentFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &nonExistentKey, proofSet, crypto.Pedersen)
		require.Equal(t, &felt.Zero, val)
		require.NoError(t, err)
	})

	t.Run("non existent leaf key", func(t *testing.T) {
		t.Parallel()

		tempTrie, _ := buildSimpleDoubleBinaryTrie(t)

		nonExistentFelt := new(felt.Felt).SetUint64(2)
		nonExistentKey := tempTrie.FeltToKey(nonExistentFelt)

		proofSet := trie.NewProofSet()
		err := tempTrie.Prove(nonExistentFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Root()
		require.NoError(t, err)

		val, err := trie.VerifyProof(root, &nonExistentKey, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, &felt.Zero, val)
	})
}

func TestProveNKeys(t *testing.T) {
	t.Parallel()

	n := 10000
	tempTrie := buildTrieWithNKeys(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i))
		key := tempTrie.FeltToKey(keyFelt)

		proofSet := trie.NewProofSet()
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

	n := 10000
	tempTrie := buildTrieWithNKeys(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i + n))
		key := tempTrie.FeltToKey(keyFelt)

		proofSet := trie.NewProofSet()
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

	fmt.Println(keys)
	for i := 0; i < n; i++ {
		key := tempTrie.FeltToKey(keys[i])

		proofSet := trie.NewProofSet()
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

func TestGetProof(t *testing.T) {
	t.Run("GP Simple Trie - simple binary", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)

		zero := trie.NewKey(250, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			&trie.Edge{
				Path:  &zero,
				Child: utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
			},

			&trie.Binary{
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

	t.Run("GP Simple Trie - simple double binary", func(t *testing.T) {
		tempTrie, expectedProofNodes := buildSimpleDoubleBinaryTrie(t)

		expectedProofNodes[2] = &trie.Binary{
			LeftHash:  utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002"),
			RightHash: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003"),
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
			&trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x06E08BF82793229338CE60B65D1845F836C8E2FBFE2BC59FF24AEDBD8BA219C4"),
				RightHash: utils.HexToFelt(t, "0x04F9B8E66212FB528C0C1BD02F43309C53B895AA7D9DC91180001BDD28A588FA"),
			},
			&trie.Edge{
				Path:  &path1,
				Child: utils.HexToFelt(t, "0xcc"),
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
			&trie.Edge{
				Path:  &path1,
				Child: child,
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

func TestProofToPath(t *testing.T) {
	t.Run("PTP Proof To Path Simple binary trie proof to path", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)
		zeroFeltByte := new(felt.Felt).Bytes()
		zero := trie.NewKey(250, zeroFeltByte[:])
		leafValue := utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000002")
		siblingValue := utils.HexToFelt(t, "0x0000000000000000000000000000000000000000000000000000000000000003")
		proofNodes := []trie.ProofNode{
			&trie.Edge{
				Path:  &zero,
				Child: utils.HexToFelt(t, "0x05774FA77B3D843AE9167ABD61CF80365A9B2B02218FC2F628494B5BDC9B33B8"),
			},
			&trie.Binary{
				LeftHash:  leafValue,
				RightHash: siblingValue,
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
			&trie.Binary{
				LeftHash:  utils.HexToFelt(t, "0x06E08BF82793229338CE60B65D1845F836C8E2FBFE2BC59FF24AEDBD8BA219C4"),
				RightHash: utils.HexToFelt(t, "0x04F9B8E66212FB528C0C1BD02F43309C53B895AA7D9DC91180001BDD28A588FA"),
			},
			&trie.Edge{
				Path:  &path1,
				Child: utils.HexToFelt(t, "0xcc"),
			},
		}

		siblingValue := utils.HexToFelt(t, "0xdd")
		sns, err := trie.ProofToPath(proofNodes, &leafkey, crypto.Pedersen)
		require.NoError(t, err)
		rootKey := tempTrie.RootKey()
		rootNode, err := tempTrie.GetNodeFromKey(rootKey)
		require.NoError(t, err)
		leftNode, err := tempTrie.GetNodeFromKey(rootNode.Left)
		require.NoError(t, err)
		require.Equal(t, 1, len(sns))
		require.Equal(t, rootKey.Len(), sns[0].Key().Len())
		require.Equal(t, leftNode.HashFromParent(rootKey, rootNode.Left, crypto.Pedersen).String(), sns[0].Node().LeftHash.String())
		require.NotEqual(t, siblingValue.String(), sns[0].Node().RightHash.String())
	})
}

// func TestVerifyRangeProof(t *testing.T) {
// 	t.Run("VPR two proofs, single key trie", func(t *testing.T) {
// 		//		Node (edge path 249)
// 		//		/			\
// 		//  Node (binary)	0x6 (leaf)
// 		//	/	\
// 		// 0x4	0x5 (leaf, leaf)

// 		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
// 		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])
// 		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
// 		twoLeafkey := trie.NewKey(251, twoFeltBytes[:])

// 		tri := build3KeyTrie(t)
// 		keys := []*felt.Felt{new(felt.Felt).SetUint64(1)}
// 		values := []*felt.Felt{new(felt.Felt).SetUint64(5)}
// 		proofKeys := [2]*trie.Key{&zeroLeafkey, &twoLeafkey}
// 		proofValues := [2]*felt.Felt{new(felt.Felt).SetUint64(4), new(felt.Felt).SetUint64(6)}
// 		rootCommitment, err := tri.Root()
// 		require.NoError(t, err)
// 		proofs, err := trie.GetBoundaryProofs(proofKeys[0], proofKeys[1], tri)
// 		require.NoError(t, err)
// 		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
// 		require.NoError(t, err)
// 		require.True(t, verif)
// 	})

// 	t.Run("VPR all keys provided, no proofs needed", func(t *testing.T) {
// 		//		Node (edge path 249)
// 		//		/			\
// 		//  Node (binary)	0x6 (leaf)
// 		//	/	\
// 		// 0x4	0x5 (leaf, leaf)
// 		tri := build3KeyTrie(t)
// 		keys := []*felt.Felt{new(felt.Felt).SetUint64(0), new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2)}
// 		values := []*felt.Felt{new(felt.Felt).SetUint64(4), new(felt.Felt).SetUint64(5), new(felt.Felt).SetUint64(6)}
// 		proofKeys := [2]*trie.Key{}
// 		proofValues := [2]*felt.Felt{}
// 		proofs := [2][]trie.ProofNode{}
// 		rootCommitment, err := tri.Root()
// 		require.NoError(t, err)
// 		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
// 		require.NoError(t, err)
// 		require.True(t, verif)
// 	})

// 	t.Run("VPR left proof, all right keys", func(t *testing.T) {
// 		//		Node (edge path 249)
// 		//		/			\
// 		//  Node (binary)	0x6 (leaf)
// 		//	/	\
// 		// 0x4	0x5 (leaf, leaf)

// 		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
// 		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])

// 		tri := build3KeyTrie(t)
// 		keys := []*felt.Felt{new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2)}
// 		values := []*felt.Felt{new(felt.Felt).SetUint64(5), new(felt.Felt).SetUint64(6)}
// 		proofKeys := [2]*trie.Key{&zeroLeafkey}
// 		proofValues := [2]*felt.Felt{new(felt.Felt).SetUint64(4)}
// 		leftProof, err := trie.GetProof(proofKeys[0], tri)
// 		require.NoError(t, err)
// 		proofs := [2][]trie.ProofNode{leftProof}
// 		rootCommitment, err := tri.Root()
// 		require.NoError(t, err)
// 		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
// 		require.NoError(t, err)
// 		require.True(t, verif)
// 	})

// 	t.Run("VPR right proof, all left keys", func(t *testing.T) {
// 		//		Node (edge path 249)
// 		//		/			\
// 		//  Node (binary)	0x6 (leaf)
// 		//	/	\
// 		// 0x4	0x5 (leaf, leaf)
// 		twoFeltBytes := new(felt.Felt).SetUint64(2).Bytes()
// 		twoLeafkey := trie.NewKey(251, twoFeltBytes[:])

// 		tri := build3KeyTrie(t)
// 		keys := []*felt.Felt{new(felt.Felt).SetUint64(0), new(felt.Felt).SetUint64(1)}
// 		values := []*felt.Felt{new(felt.Felt).SetUint64(4), new(felt.Felt).SetUint64(5)}
// 		proofKeys := [2]*trie.Key{nil, &twoLeafkey}
// 		proofValues := [2]*felt.Felt{nil, new(felt.Felt).SetUint64(6)}
// 		rightProof, err := trie.GetProof(proofKeys[1], tri)
// 		require.NoError(t, err)
// 		proofs := [2][]trie.ProofNode{nil, rightProof}
// 		rootCommitment, err := tri.Root()
// 		require.NoError(t, err)
// 		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
// 		require.NoError(t, err)
// 		require.True(t, verif)
// 	})

// 	t.Run("VPR left proof, all inner keys, right proof with non-set key", func(t *testing.T) {
// 		zeroFeltBytes := new(felt.Felt).SetUint64(0).Bytes()
// 		zeroLeafkey := trie.NewKey(251, zeroFeltBytes[:])

// 		threeFeltBytes := new(felt.Felt).SetUint64(3).Bytes()
// 		threeLeafkey := trie.NewKey(251, threeFeltBytes[:])

// 		tri := build4KeyTrie(t)
// 		keys := []*felt.Felt{new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2)}
// 		values := []*felt.Felt{new(felt.Felt).SetUint64(5), new(felt.Felt).SetUint64(6)}
// 		proofKeys := [2]*trie.Key{&zeroLeafkey, &threeLeafkey}
// 		proofValues := [2]*felt.Felt{new(felt.Felt).SetUint64(4), nil}
// 		leftProof, err := trie.GetProof(proofKeys[0], tri)
// 		require.NoError(t, err)
// 		rightProof, err := trie.GetProof(proofKeys[1], tri)
// 		require.NoError(t, err)

// 		proofs := [2][]trie.ProofNode{leftProof, rightProof}
// 		rootCommitment, err := tri.Root()
// 		require.NoError(t, err)

// 		verif, err := trie.VerifyRangeProof(rootCommitment, keys, values, proofKeys, proofValues, proofs, crypto.Pedersen)
// 		require.NoError(t, err)
// 		require.True(t, verif)
// 	})
// }

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

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
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
