package triedb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var (
	_ database.NodeDatabase = (*EmptyNodeDatabase)(nil)
	_ database.NodeReader   = (*EmptyNodeReader)(nil)
)

type EmptyNodeDatabase struct{}

func NewEmptyNodeDatabase() *EmptyNodeDatabase {
	return &EmptyNodeDatabase{}
}

func (EmptyNodeDatabase) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &EmptyNodeReader{}, nil
}

type EmptyNodeReader struct{}

func (EmptyNodeReader) Node(
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Felt,
	isLeaf bool,
) ([]byte, error) {
	return nil, nil
}
