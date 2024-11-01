package trie_test

import (
	"bytes"
	"encoding/hex"
	"errors"
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

func TestNodeErrorHandling(t *testing.T) {
	t.Run("WriteTo node value is nil", func(t *testing.T) {
		node := trie.Node{}
		var buffer bytes.Buffer
		_, err := node.WriteTo(&buffer)
		require.Error(t, err)
	})
	t.Run("WriteTo returns error", func(t *testing.T) {
		node := trie.Node{
			Value: new(felt.Felt).SetUint64(42),
			Left:  &trie.Key{},
			Right: &trie.Key{},
		}

		wrote, err := node.WriteTo(&errorBuffer{})
		require.Error(t, err)
		require.Equal(t, int64(0), wrote)
	})
	t.Run("UnmarshalBinary returns error", func(t *testing.T) {
		node := trie.Node{}

		err := node.UnmarshalBinary([]byte{42})
		require.Equal(t, errors.New("size of input data is less than felt size"), err)

		bs := new(felt.Felt).Bytes()
		err = node.UnmarshalBinary(append(bs[:], 0, 0, 42))
		require.Equal(t, errors.New("the node does not contain both left and right hash"), err)
	})
}
