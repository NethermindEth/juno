package trie

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
)

func TestNode_Marshall(t *testing.T) {
	value, _ := new(felt.Felt).SetRandom()
	path1 := bitset.FromWithLength(44, []uint64{44})
	path2 := bitset.FromWithLength(22, []uint64{22})

	tests := [...]Node{
		{
			value: value,
			left:  nil,
			right: nil,
		},
		{
			value: value,
			left:  path1,
			right: nil,
		},
		{
			value: value,
			left:  nil,
			right: path2,
		},
		{
			value: value,
			left:  path1,
			right: path2,
		},
		{
			value: value,
			left:  path2,
			right: path1,
		},
	}
	for _, test := range tests {
		data, _ := test.MarshalBinary()
		unmarshaled := new(Node)
		_ = unmarshaled.UnmarshalBinary(data)
		if !test.Equal(unmarshaled) {
			t.Error("TestNode_Marshall failed")
		}
	}

	malformed := new([felt.Bytes + 1]byte)
	malformed[felt.Bytes] = 'l'
	if err := new(Node).UnmarshalBinary(malformed[2:]); err == nil {
		t.Error("TestNode_Marshall failed")
	}
	if err := new(Node).UnmarshalBinary(malformed[:]); err == nil {
		t.Error("TestNode_Marshall failed")
	}
	malformed[felt.Bytes] = 'z'
	if err := new(Node).UnmarshalBinary(malformed[:]); err == nil {
		t.Error("TestNode_Marshall failed")
	}
}

func TestTrieNode_Hash(t *testing.T) {
	// https://github.com/eqlabs/pathfinder/blob/5e0f4423ed9e9385adbe8610643140e1a82eaef6/crates/pathfinder/src/state/merkle_node.rs#L350-L374
	valueBytes, _ := hex.DecodeString("1234ABCD")
	expected, _ := new(felt.Felt).SetString("0x1d937094c09b5f8e26a662d21911871e3cbc6858d55cc49af9848ea6fed4e9")

	node := Node{
		value: new(felt.Felt).SetBytes(valueBytes),
	}
	path := bitset.FromWithLength(6, []uint64{42})

	assert.Equal(t, true, expected.Equal(node.Hash(path)), "TestTrieNode_Hash failed")
}
