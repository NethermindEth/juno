package pathdb

import (
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/core/felt"
)

type (
	classNodesMap           = map[trieutils.Path]trienode.TrieNode
	contractNodesMap        = map[trieutils.Path]trienode.TrieNode
	contractStorageNodesMap = map[felt.Felt]map[trieutils.Path]trienode.TrieNode
)

const ownerSize = felt.Bytes
