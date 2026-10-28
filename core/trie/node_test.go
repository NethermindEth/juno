package trie_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeHash(t *testing.T) {
	// https://github.com/eqlabs/pathfinder/blob/5e0f4423ed9e9385adbe8610643140e1a82eaef6/crates/pathfinder/src/state/merkle_node.rs#L350-L374
	valueBytes, err := hex.DecodeString("1234ABCD")
	require.NoError(t, err)

	expected := felt.NewUnsafeFromString[felt.Felt]("0x1d937094c09b5f8e26a662d21911871e3cbc6858d55cc49af9848ea6fed4e9")

	node := trie.Node{
		Value: new(felt.Felt).SetBytes(valueBytes),
	}
	path := trie.NewBitArray(6, 42)
	hash := node.Hash(&path, crypto.Pedersen)
	assert.Equal(t, expected, &hash, "TestTrieNode_Hash failed")
}

func encodeNode(t *testing.T, node *trie.Node) []byte {
	t.Helper()
	var buffer bytes.Buffer
	_, err := node.WriteTo(&buffer)
	require.NoError(t, err)
	return buffer.Bytes()
}

func TestNodeEncoding(t *testing.T) {
	leafValues := []felt.Felt{
		felt.Zero,
		felt.FromUint64[felt.Felt](7),
		felt.FromUint64[felt.Felt](1 << 40),
		felt.UnsafeFromString[felt.Felt](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		),
	}

	t.Run("leaf stores its value in the build's form", func(t *testing.T) {
		for _, value := range leafValues {
			encoded := encodeNode(t, &trie.Node{Value: &value})
			full := value.Bytes()
			if felt.CompactWrites {
				require.Equal(t, value.MinimalBytes(), encoded)
			} else {
				require.Equal(t, full[:], encoded)
			}

			var decoded trie.Node
			require.NoError(t, decoded.UnmarshalBinary(encoded))
			require.Equal(t, value, *decoded.Value)
			require.Nil(t, decoded.Left)
			require.Nil(t, decoded.LeftHash)
		}
	})

	t.Run("every build decodes both leaf forms", func(t *testing.T) {
		for _, value := range leafValues {
			full := value.Bytes()
			for _, stored := range [][]byte{full[:], value.MinimalBytes()} {
				var decoded trie.Node
				require.NoError(t, decoded.UnmarshalBinary(stored))
				require.Equal(t, value, *decoded.Value)
				require.Nil(t, decoded.Left)
			}
		}
	})

	t.Run("internal node keeps its full encoding", func(t *testing.T) {
		left, right := trie.NewBitArray(3, 5), trie.NewBitArray(0, 0)
		value := felt.FromUint64[felt.Felt](9)
		leftHash, rightHash := felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)
		node := trie.Node{
			Value: &value, Left: &left, Right: &right, LeftHash: &leftHash, RightHash: &rightHash,
		}
		encoded := encodeNode(t, &node)
		require.Greater(t, len(encoded), felt.Bytes)
		raw := value.Bytes()
		require.Equal(t, raw[:], encoded[:felt.Bytes])

		var decoded trie.Node
		require.NoError(t, decoded.UnmarshalBinary(encoded))
		require.Equal(t, node, decoded)

		// A reused node decoding a leaf drops the previous children.
		require.NoError(t, decoded.UnmarshalBinary([]byte{3}))
		require.Equal(t, felt.FromUint64[felt.Felt](3), *decoded.Value)
		require.Nil(t, decoded.Left)
		require.Nil(t, decoded.Right)
		require.Nil(t, decoded.LeftHash)
		require.Nil(t, decoded.RightHash)
	})

	t.Run("rejects empty data", func(t *testing.T) {
		var decoded trie.Node
		require.Error(t, decoded.UnmarshalBinary(nil))
	})
}
