package pathdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type leafType uint8

const (
	nonLeaf leafType = iota + 1
	leaf
)

func (l leafType) Bytes() []byte {
	return []byte{byte(l)}
}

type (
	classNodesMap           = map[trieutils.Path]trienode.TrieNode
	contractNodesMap        = map[trieutils.Path]trienode.TrieNode
	contractStorageNodesMap = map[felt.Felt]map[trieutils.Path]trienode.TrieNode
)
