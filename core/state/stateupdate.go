package state

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
)

var emptyStateUpdate = stateUpdate{}

type stateUpdate struct {
	prevComm felt.Felt // state commitment before applying updates
	curComm  felt.Felt // state commitment after applying updates

	classNodes    *trienode.MergeNodeSet // class trie nodes to be updated
	contractNodes *trienode.MergeNodeSet // contract trie nodes to be updated
}
