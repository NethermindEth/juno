package rawdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type (
	classNodesMap           = map[trieutils.Path]trienode.TrieNode
	contractNodesMap        = map[trieutils.Path]trienode.TrieNode
	contractStorageNodesMap = map[felt.Felt]map[trieutils.Path]trienode.TrieNode
)
