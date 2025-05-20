package state

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var emptyStateUpdate = stateUpdate{}

type stateUpdate struct {
	prevComm felt.Felt // state commitment before applying updates
	curComm  felt.Felt // state commitment after applying updates

	classNodes    map[trieutils.Path]trienode.TrieNode               // class trie nodes to be updated
	contractNodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode // contract trie nodes to be updated
}
