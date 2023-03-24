package trie

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
)

func TestNodeHash(t *testing.T) {
	// https://github.com/eqlabs/pathfinder/blob/5e0f4423ed9e9385adbe8610643140e1a82eaef6/crates/pathfinder/src/state/merkle_node.rs#L350-L374
	valueBytes, _ := hex.DecodeString("1234ABCD")
	expected, _ := new(felt.Felt).SetString("0x1d937094c09b5f8e26a662d21911871e3cbc6858d55cc49af9848ea6fed4e9")

	node := Node{
		Value: new(felt.Felt).SetBytes(valueBytes),
	}
	path := bitset.FromWithLength(6, []uint64{42})

	assert.Equal(t, expected, node.Hash(path, crypto.Pedersen), "TestTrieNode_Hash failed")
}
