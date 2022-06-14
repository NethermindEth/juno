package trie

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/types"
)

// node represents a node in a binary tree.
type node struct {
	length int
	path   types.Felt
	bottom types.Felt
}

func (n *node) IsPrefix(key *types.Felt) bool {
	for i := 0; i < n.length; i++ {
		if n.path.Big().Bit(i) != key.Big().Bit(i) {
			return false
		}
	}
	return true
}

func (n *node) MarshallJSON() ([]byte, error) {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{n.length, n.path.Hex(), n.bottom.Hex()}
	return json.Marshal(jsonNode)
}

func (n *node) UnmarshalJSON(b []byte) error {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{}

	if err := json.Unmarshal(b, &jsonNode); err != nil {
		return err
	}

	n.length = jsonNode.Length
	n.path = types.HexToFelt(jsonNode.Path)
	n.bottom = types.HexToFelt(jsonNode.Bottom)
	return nil
}
