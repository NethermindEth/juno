package pathdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type layer interface {
	node(owner felt.Felt, path trieutils.Path, isClass bool) ([]byte, error)
	update(root felt.Felt, id, block uint64, nodes *nodeSet) *diffLayer
	journal() error
	rootHash() felt.Felt
	stateID() uint64
}

type layerTree struct {
	layers []*diffLayer
}
