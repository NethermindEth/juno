package pathdb

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ database.NodeReader = (*reader)(nil)

type reader struct {
	id trieutils.TrieID
	l  layer
}

func (r *reader) Node(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	return r.l.node(r.id, owner, path, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	l := d.tree.get(id.StateComm())
	if l == nil {
		return nil, fmt.Errorf("layer %v not found", id.StateComm())
	}
	return &reader{id: id, l: l}, nil
}
