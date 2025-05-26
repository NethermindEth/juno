package pathdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ database.NodeReader = (*reader)(nil)

// TODO(weiihann): eventually use the layertree for this
type reader struct {
	id trieutils.TrieID
	db *Database
}

func (r *reader) Node(owner *felt.Felt, path *trieutils.Path, hash *felt.Felt, isLeaf bool) ([]byte, error) {
	return trieutils.GetNodeByPath(r.db.disk, r.id.Bucket(), owner, path, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{db: d, id: id}, nil
}
