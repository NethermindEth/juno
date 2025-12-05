package rawdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var _ database.TrieDB = (*Database)(nil)

type Config struct{}

type Database struct {
	disk db.KeyValueStore

	lock sync.RWMutex
	log  utils.SimpleLogger
}

func New(disk db.KeyValueStore) *Database {
	return &Database{
		disk: disk,
		log:  utils.NewNopZapLogger(),
	}
}

func (d *Database) readNode(
	id trieutils.TrieID,
	owner *felt.Address,
	path *trieutils.Path,
	isLeaf bool,
) ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	blob, err := trieutils.GetNodeByPath(d.disk, id.Bucket(), owner, path, isLeaf)
	if err != nil {
		return nil, err
	}

	return blob, nil
}

func (d *Database) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	key := id.Bucket().Key()
	owner := id.Owner()
	if !felt.IsZero(&owner) {
		oBytes := owner.Bytes()
		key = append(key, oBytes[:]...)
	}

	return d.disk.NewIterator(key, true)
}

func (d *Database) Update(
	root,
	parent *felt.Hash,
	blockNum uint64,
	mergedClassNodes *trienode.MergeNodeSet,
	mergedContractNodes *trienode.MergeNodeSet,
) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	batch := d.disk.NewBatch()

	var classNodes classNodesMap
	var contractNodes contractNodesMap
	var contractStorageNodes contractStorageNodesMap

	if mergedClassNodes != nil {
		classNodes, _ = mergedClassNodes.Flatten()
	}
	if mergedContractNodes != nil {
		contractNodes, contractStorageNodes = mergedContractNodes.Flatten()
	}

	for path, n := range classNodes {
		err := d.updateNode(batch, db.ClassTrie, &felt.Address{}, &path, n)
		if err != nil {
			return err
		}
	}

	for path, n := range contractNodes {
		err := d.updateNode(batch, db.ContractTrieContract, &felt.Address{}, &path, n)
		if err != nil {
			return err
		}
	}

	for owner, nodes := range contractStorageNodes {
		for path, n := range nodes {
			err := d.updateNode(batch, db.ContractTrieStorage, &owner, &path, n)
			if err != nil {
				return err
			}
		}
	}

	return batch.Write()
}

func (d *Database) updateNode(
	batch db.KeyValueWriter,
	bucket db.Bucket,
	owner *felt.Address,
	path *trieutils.Path,
	n trienode.TrieNode,
) error {
	if _, deleted := n.(*trienode.DeletedNode); deleted {
		return trieutils.DeleteNodeByPath(batch, bucket, owner, path, n.IsLeaf())
	}
	return trieutils.WriteNodeByPath(
		batch,
		bucket,
		owner,
		path,
		n.IsLeaf(),
		n.Blob(),
	)
}

// This method was added to satisfy the TrieDB interface, but it is not used.
func (d *Database) Commit(_ *felt.Hash) error {
	return nil
}

// This method was added to satisfy the TrieDB interface, but it is not used.
func (d *Database) Close() error {
	return nil
}

func (d *Database) Scheme() database.TrieDBScheme {
	return database.RawScheme
}
