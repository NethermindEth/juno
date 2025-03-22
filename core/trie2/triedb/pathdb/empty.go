package pathdb

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var _ database.TrieDB = (*EmptyDatabase)(nil)

var ErrCallEmptyDatabase = errors.New("call to empty disk")

type EmptyDatabase struct{}

func (EmptyDatabase) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return nil, ErrCallEmptyDatabase
}

func (EmptyDatabase) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	return nil, ErrCallEmptyDatabase
}

func (EmptyDatabase) Close() error {
	return ErrCallEmptyDatabase
}

func (EmptyDatabase) Commit(stateRoot felt.Felt) error {
	return ErrCallEmptyDatabase
}
