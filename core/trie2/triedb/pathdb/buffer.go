package pathdb

import (
	"fmt"

	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

// Stores the pending nodes in memory to be committed later
type buffer struct {
	layers uint64 // number of layers merged into the buffer
	limit  uint64 // maximum number of nodes the buffer can hold
	nodes  *nodeSet
}

func newBuffer(limit int, nodes *nodeSet, layer uint64) *buffer {
	if nodes == nil {
		nodes = newNodeSet(nil, nil, nil)
	}
	return &buffer{
		limit:  uint64(limit),
		nodes:  nodes,
		layers: layer,
	}
}

func (b *buffer) node(owner *felt.Felt, path *trieutils.Path, isClass bool) (trienode.TrieNode, bool) {
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

func (b *buffer) isFull() bool {
	return b.nodes.size > b.limit
}

func (b *buffer) flush(kvs db.KeyValueStore, cleans *cleanCache, id uint64) error {
	latestPersistedID, _ := trieutils.ReadPersistedStateID(kvs)
	if latestPersistedID+b.layers != id {
		return fmt.Errorf(
			"mismatch buffer layers applied: latest state id (%d) + buffer layers (%d) != target state id (%d)",
			latestPersistedID,
			b.layers,
			id,
		)
	}

	batch := kvs.NewBatchWithSize(b.nodes.dbSize())
	if err := b.nodes.write(batch, cleans); err != nil {
		return err
	}
	if err := trieutils.WritePersistedStateID(batch, id); err != nil {
		return err
	}

	if err := batch.Write(); err != nil {
		return err
	}

	b.reset()
	return nil
}
