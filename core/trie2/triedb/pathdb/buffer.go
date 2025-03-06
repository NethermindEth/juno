package pathdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type buffer struct {
	layers uint64
	limit  uint64
	nodes  *nodeSet
}

func (b *buffer) node(owner felt.Felt, path trieutils.Path, isClass bool) (trienode.TrieNode, bool) {
	return b.nodes.node(owner, path, isClass)
}

func (b *buffer) commit(nodes *nodeSet) *buffer {
	b.layers++
	b.nodes.merge(nodes)
	return b
}

func (b *buffer) reset() {
	b.layers = 0
	b.limit = 0
	b.nodes.reset()
}

func (b *buffer) size() uint64 {
	return b.nodes.size
}

func (b *buffer) isFull() bool {
	return b.nodes.size > b.limit
}

func (b *buffer) flush(db db.DB, id uint64) error {
	// TODO(weiihann): need to double check internal counter?
	// TODO(weiihann): store internal counter?
	panic("TODO(weiihann): implement me")
}
