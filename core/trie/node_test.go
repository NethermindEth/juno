package trie_test

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeHash(t *testing.T) {
	// https://github.com/eqlabs/pathfinder/blob/5e0f4423ed9e9385adbe8610643140e1a82eaef6/crates/pathfinder/src/state/merkle_node.rs#L350-L374
	valueBytes, err := hex.DecodeString("1234ABCD")
	require.NoError(t, err)

	expected := utils.HexToFelt(t, "0x1d937094c09b5f8e26a662d21911871e3cbc6858d55cc49af9848ea6fed4e9")

	node := trie.Node{
		Value: new(felt.Felt).SetBytes(valueBytes),
	}
	path := trie.NewKey(6, []byte{42})

	assert.Equal(t, expected, node.Hash(&path, crypto.Pedersen), "TestTrieNode_Hash failed")
}
