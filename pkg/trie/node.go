package trie

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

// Node represents a Node in a binary tree.
type Node struct {
	Path   *Path
	Bottom *types.Felt
}

func (n *Node) Hash() *types.Felt {
	if n.Path.Len() == 0 {
		return n.Bottom
	}
	// TODO: why does `pedersen.Digest` operates with `big.Int`
	//       this should be changed to `types.Felt`
	h := types.BigToFelt(pedersen.Digest(n.Bottom.Big(), new(big.Int).SetBytes(n.Path.Bytes())))
	length := types.BigToFelt(new(big.Int).SetUint64(uint64(n.Path.Len())))
	felt := h.Add(length)
	return &felt
}

func (n *Node) MarshallJSON() ([]byte, error) {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{n.Path.Len(), hex.EncodeToString(n.Path.Bytes()), n.Bottom.Hex()}
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

	path, err := hex.DecodeString(jsonNode.Path)
	if err != nil {
		return err
	}
	n.Path = NewPath(jsonNode.Length, path)
	*n.Bottom = types.HexToFelt(jsonNode.Bottom)
	return nil
}
