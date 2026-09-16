package trie

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestProveMatchesLegacySingleKeyProof(t *testing.T) {
	t.Parallel()

	memdb := memory.New()
	txn := memdb.NewIndexedBatch()
	tempTrie, err := NewTriePedersen(txn, []byte{0}, 251)
	require.NoError(t, err)

	keys := []*felt.Felt{
		new(felt.Felt).SetUint64(1),
		new(felt.Felt).SetUint64(42),
		new(felt.Felt).SetUint64(128),
		new(felt.Felt).SetUint64(255),
	}
	for i, key := range keys {
		_, err := tempTrie.Put(key, new(felt.Felt).SetUint64(uint64(i+1)))
		require.NoError(t, err)
	}
	require.NoError(t, tempTrie.Commit())

	proofKeys := append(keys, new(felt.Felt).SetUint64(999))
	for _, key := range proofKeys {
		legacyProof := NewProofNodeSet()
		require.NoError(t, proveLegacyForTest(tempTrie, key, legacyProof))

		proof := NewProofNodeSet()
		require.NoError(t, tempTrie.Prove(key, proof))

		requireProofNodeSetEqualForTest(t, legacyProof, proof, crypto.Pedersen)
	}
}

func proveLegacyForTest(t *Trie, key *felt.Felt, proof *ProofNodeSet) error {
	if err := t.ensureNoUnhashedWrites(); err != nil {
		return err
	}

	trieKey := t.FeltToKey(key)
	nodesFromRoot, err := t.nodesFromRoot(&trieKey)
	if err != nil {
		return err
	}

	var parentKey *BitArray
	var carriedHash *felt.Felt

	for i, storageNode := range nodesFromRoot {
		isLeaf := storageNode.key.len == t.height

		var onPathChild *StorageNode
		if !isLeaf && i+1 < len(nodesFromRoot) {
			onPathChild = &nodesFromRoot[i+1]
		}
		binary, err := t.addProofNode(parentKey, storageNode, carriedHash, proof, onPathChild)
		if err != nil {
			return err
		}

		if isLeaf {
			break
		}

		carriedHash = nil
		switch {
		case onPathChild == nil:
		case onPathChild.key.Equal(storageNode.node.Left):
			carriedHash = binary.LeftHash
		case onPathChild.key.Equal(storageNode.node.Right):
			carriedHash = binary.RightHash
		}
		parentKey = storageNode.key
	}
	return nil
}

func requireProofNodeSetEqualForTest(
	t *testing.T,
	expected *ProofNodeSet,
	actual *ProofNodeSet,
	hash crypto.HashFn,
) {
	t.Helper()

	require.Equal(t, expected.Size(), actual.Size())
	for _, key := range expected.Keys() {
		expectedNode, ok := expected.Get(key)
		require.True(t, ok)

		actualNode, ok := actual.Get(key)
		require.True(t, ok)

		require.Equal(t, expectedNode.Hash(hash), actualNode.Hash(hash))
		require.Equal(t, expectedNode.String(), actualNode.String())
	}
}
