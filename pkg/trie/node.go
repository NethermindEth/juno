package trie

import (
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

// Node represents a Node in a binary tree.
type Node struct {
	Length int
	Path   types.Felt
	Bottom types.Felt
}

func (n *Node) hash() types.Felt {
	if n.Length == 0 {
		return n.Bottom
	}
	// TODO: why does `pedersen.Digest` operates with `big.Int`
	//       this should be changed to `types.Felt`
	h := pedersen.Digest(n.Bottom.Big(), n.Path.Big())
	return types.BigToFelt(h.Add(h, big.NewInt(int64(n.Length)))) // TODO: add modulo with P here
}

func (n *Node) isPrefix(key *types.Felt) bool {
	for i := uint(0); i < uint(n.Length); i++ {
		if n.Path.Bit(i) != key.Bit(i) {
			return false
		}
	}
	return true
}

func (n *Node) MarshallJSON() ([]byte, error) {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{n.Length, n.Path.Hex(), n.Bottom.Hex()}
	return json.Marshal(jsonNode)
}

func (n *Node) UnmarshalJSON(b []byte) error {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{}

	if err := json.Unmarshal(b, &jsonNode); err != nil {
		return err
	}

	n.Length = jsonNode.Length
	n.Path = types.HexToFelt(jsonNode.Path)
	n.Bottom = types.HexToFelt(jsonNode.Bottom)
	return nil
}
