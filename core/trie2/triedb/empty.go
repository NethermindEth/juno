package triedb

import (
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ database.NodeDatabase = (*EmptyNodeDatabase)(nil)

type EmptyNodeDatabase struct{}

func NewEmptyNodeDatabase() *EmptyNodeDatabase {
	return &EmptyNodeDatabase{}
}

func (EmptyNodeDatabase) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return nil, nil
}
