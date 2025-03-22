package pathdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var _ database.TrieDB = (*Database)(nil)

type Config struct{} // TODO(weiihann): handle this

type Database struct {
	disk db.KeyValueStore
	// TODO(weiihann): add the cache stuff here
	config *Config
}

func New(disk db.KeyValueStore, config *Config) *Database {
	return &Database{disk: disk, config: config}
}

func (d *Database) Close() error {
	panic("TODO(weiihann): implement me")
}

func (d *Database) Commit(stateRoot felt.Felt) error {
	panic("TODO(weiihann): implement me")
}

// TODO(weiihann): how to deal with stateroot and the layer tree?
func (d *Database) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	var (
		idBytes    = id.Bucket().Key()
		ownerBytes []byte
	)

	owner := id.Owner()
	if !owner.Equal(&felt.Zero) {
		ob := owner.Bytes()
		ownerBytes = ob[:]
	}

	prefix := make([]byte, 0, len(idBytes)+len(ownerBytes))
	prefix = append(prefix, idBytes...)
	prefix = append(prefix, ownerBytes...)

	return d.disk.NewIterator(prefix, true)
}
