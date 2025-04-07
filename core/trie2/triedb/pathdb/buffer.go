package pathdb

import (
	"fmt"

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

func (b *buffer) flush(d db.KeyValueStore, cleans *cleanCache, id uint64) error {
	latestPersistedID, err := trieutils.ReadPersistedStateID(d)
	if err != nil {
		return err
	}

	if latestPersistedID+b.layers != id {
		return fmt.Errorf(
			"mismatch buffer layers applied: latest state id (%d) + buffer layers (%d) != target state id (%d)",
			latestPersistedID,
			b.layers,
			id,
		)
	}

	batch := d.NewBatchWithSize(b.nodes.dbSize())
	b.nodes.write(batch, cleans)
	if err := trieutils.WritePersistedStateID(batch, id); err != nil {
		return err
	}

	if err := batch.Write(); err != nil {
		return err
	}

	return nil
}
