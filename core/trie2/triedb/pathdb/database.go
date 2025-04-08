package pathdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var _ database.TrieDB = (*Database)(nil)

type Config struct {
	CleanCacheSize int // Maximum size (in bytes) for caching clean nodes
} // TODO(weiihann): handle this

type Database struct {
	disk db.KeyValueStore
	// TODO(weiihann): add the cache stuff here
	tree   *layerTree
	config *Config
	lock   sync.RWMutex
}

func New(disk db.KeyValueStore, config *Config) *Database {
	return &Database{disk: disk, config: config}
}

func (d *Database) Close() error {
	panic("TODO(weiihann): implement me")
}

func (d *Database) Commit(stateComm felt.Felt) error {
	panic("TODO(weiihann): implement me")
}

// TODO(weiihann): how to deal with state comm and the layer tree?
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

func (d *Database) Update(
	root,
	parent felt.Felt,
	blockNum uint64,
	classNodes map[trieutils.Path]trienode.TrieNode,
	contractNodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode,
) error {
	batch := d.disk.NewBatch()

	for path, node := range classNodes {
		if _, ok := node.(*trienode.DeletedNode); ok {
			if err := trieutils.DeleteNodeByPath(batch, db.ClassTrie, felt.Zero, path, node.IsLeaf()); err != nil {
				return err
			}
		} else {
			if err := trieutils.WriteNodeByPath(batch, db.ClassTrie, felt.Zero, path, node.IsLeaf(), node.Blob()); err != nil {
				return err
			}
		}
	}

	for owner, nodes := range contractNodes {
		bucket := db.ContractTrieStorage
		if owner.Equal(&felt.Zero) {
			bucket = db.ContractTrieContract
		}

		for path, node := range nodes {
			if _, ok := node.(*trienode.DeletedNode); ok {
				if err := trieutils.DeleteNodeByPath(batch, bucket, owner, path, node.IsLeaf()); err != nil {
					return err
				}
			} else {
				if err := trieutils.WriteNodeByPath(batch, bucket, owner, path, node.IsLeaf(), node.Blob()); err != nil {
					return err
				}
			}
		}
	}

	return batch.Write()
}
