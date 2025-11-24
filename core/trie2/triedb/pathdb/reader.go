package pathdb

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ database.NodeReader = (*reader)(nil)

// Represents a node reader which allows for node reading starting from a given layer
type reader struct {
	id trieutils.TrieID
	l  layer
}

func (r *reader) Node(
	owner *felt.Address, path *trieutils.Path, hash *felt.Felt, isLeaf bool,
) ([]byte, error) {
	return r.l.node(r.id, owner, path, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	stateComm := id.StateComm()
	l := d.tree.get(&stateComm)
	if l == nil {
		return nil, fmt.Errorf("layer %v not found", &stateComm)
	}
	return &reader{id: id, l: l}, nil
}
