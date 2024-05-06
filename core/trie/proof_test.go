package trie_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildSimpleTrie(t *testing.T) *trie.Trie {
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

// func getProofNodeBinary(t *testing.T, tri *trie.Trie, node *trie.Node) trie.ProofNode {
// 	getHash := func(tri *trie.Trie, key *trie.Key) (*felt.Felt, error) {
// 		keyFelt := key.Felt()
// 		node2, err := tri.GetNode(&keyFelt)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return node2.Hash(key, crypto.Pedersen), nil
// 	}

// 	left, err := getHash(tri, node.Left)
// 	require.NoError(t, err)
// 	right, err := getHash(tri, node.Right)
// 	require.NoError(t, err)

// 	return trie.ProofNode{
// 		Binary: &trie.Binary{
// 			LeftHash: left, RightHash: right},
// 	}

// }

func TestGetProofs(t *testing.T) {
	t.Run("Simple Trie - simple binary", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)
		zero := trie.NewKey(250, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero, // Todo: pathfinder returns 0? But shouldn't be zero?...
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
		for _, pNode := range proofNodes {
			pNode.PrettyPrint()
		}
		require.Equal(t, len(expectedProofNodes), len(proofNodes))
		require.Equal(t, expectedProofNodes, proofNodes)
	})
}

func TestVerifyProofs(t *testing.T) {
	t.Run("Simple Trie", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)
		zero := trie.NewKey(250, []byte{0})
		expectedProofNodes := []trie.ProofNode{
			{
				Edge: &trie.Edge{
					Path:  &zero, // Todo: pathfinder returns 0? But shouldn't be zero?...
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
		assert.True(t, trie.VerifyProof(root, &key1, val1, expectedProofNodes))
	})
}
