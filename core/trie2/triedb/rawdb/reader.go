package rawdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ database.NodeReader = (*reader)(nil)

type reader struct {
	id trieutils.TrieID
	d  *Database
}

func (r *reader) Node(
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Hash,
	isLeaf bool,
) ([]byte, error) {
	return r.d.readNode(r.id, owner, path, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{d: d, id: id}, nil
}
