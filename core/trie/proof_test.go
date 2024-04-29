package trie_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db/pebble"
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
	key1 := new(felt.Felt).SetUint64(1)
	key2 := new(felt.Felt).SetUint64(2)
	value1 := new(felt.Felt).SetUint64(123)
	value2 := new(felt.Felt).SetUint64(456)

	_, err = tempTrie.Put(key1, value1)
	require.NoError(t, err)

	_, err = tempTrie.Put(key2, value2)
	require.NoError(t, err)

	require.NoError(t, tempTrie.Commit())
	return tempTrie
}

func getProofNode(t *testing.T, tri *trie.Trie, node *trie.Node) trie.ProofNode {
	getHash := func(tri *trie.Trie, key *trie.Key) (*felt.Felt, error) {
		keyFelt := key.Felt()
		node2, err := tri.GetNode(&keyFelt)
		if err != nil {
			return nil, err
		}
		return node2.Hash(key, crypto.Pedersen), nil
	}

	left, err := getHash(tri, node.Left)
	require.NoError(t, err)
	right, err := getHash(tri, node.Right)
	require.NoError(t, err)

	return trie.ProofNode{LeftHash: left, RightHash: right}
}

func TestGetProofs(t *testing.T) {
	t.Run("Simple Trie", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)

		rootNode, err := tempTrie.GetRootNode()
		require.NoError(t, err)
		rootProofNode := getProofNode(t, tempTrie, rootNode)
		expectedProofNodes := []trie.ProofNode{rootProofNode}

		proofNodes, err := trie.GetProof(new(felt.Felt).SetUint64(1), tempTrie)
		require.NoError(t, err)

		require.Equal(t, expectedProofNodes, proofNodes)
	})
}

func TestVerifyProofs(t *testing.T) {
	t.Run("Simple Trie", func(t *testing.T) {
		tempTrie := buildSimpleTrie(t)

		rootNode, err := tempTrie.GetRootNode()
		require.NoError(t, err)
		rootNodeProof := getProofNode(t, tempTrie, rootNode)

		key1 := new(felt.Felt).SetUint64(1)
		key1Bytes := key1.Bytes()
		key1Key := trie.NewKey(251, key1Bytes[:])

		proofNodes := []trie.ProofNode{rootNodeProof}
		root := crypto.Pedersen(rootNodeProof.LeftHash, rootNodeProof.RightHash)

		verifiedKey1, err := trie.VerifyProof(root, &key1Key, *rootNodeProof.LeftHash, proofNodes, crypto.Pedersen)
		assert.NoError(t, err)

		assert.Equal(t, trie.Member, verifiedKey1)
	})
}
