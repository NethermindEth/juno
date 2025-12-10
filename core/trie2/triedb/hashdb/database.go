package hashdb

import (
	"fmt"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var _ database.TrieDB = (*Database)(nil)

type Config struct {
	CleanCacheSize uint64 // Maximum size (in bytes) for caching clean nodes
}

type Database struct {
	disk   db.KeyValueStore
	config Config

	cleanCache *cleanCache
	dirtyCache *dirtyCache

	lock sync.RWMutex
	log  utils.SimpleLogger
}

// Creates a new hash-based database. If the config is not provided, it will use the default config,
// which is 16MB for clean cache.
func New(disk db.KeyValueStore, config *Config) *Database {
	if config == nil {
		config = &Config{
			CleanCacheSize: 1024 * 1024 * 16,
		}
	}
	cleanCache := newCleanCache(config.CleanCacheSize)
	return &Database{
		disk:       disk,
		config:     *config,
		cleanCache: &cleanCache,
		dirtyCache: newDirtyCache(),
		log:        utils.NewNopZapLogger(),
	}
}

func (d *Database) insert(
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Felt,
	isClass bool,
	node trienode.TrieNode,
) {
	_, found := d.dirtyCache.getNode(owner, path, hash, isClass)
	if found {
		return
	}
	d.dirtyCache.putNode(owner, path, hash, isClass, node)
}

func (d *Database) readNode(
	bucket db.Bucket,
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Felt,
	isLeaf bool,
) ([]byte, error) {
	if blob := d.cleanCache.getNode(path, hash); blob != nil {
		return blob, nil
	}
	node, found := d.dirtyCache.getNode(owner, path, hash, bucket == db.ClassTrie)
	if found {
		return node.Blob(), nil
	}

	var blob []byte
	blob, err := trieutils.GetNodeByHash(d.disk, bucket, owner, path, hash, isLeaf)
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, db.ErrKeyNotFound
	}

	d.cleanCache.putNode(path, hash, blob)

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

func (d *Database) Commit(_ *felt.Felt) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	batch := d.disk.NewBatch()
	nodes, startTime := d.dirtyCache.len(), time.Now()

	for key, node := range d.dirtyCache.classNodes {
		path, hash, err := decodeNodeKey([]byte(key))
		if err != nil {
			return err
		}
		err = trieutils.WriteNodeByHash(
			batch,
			db.ClassTrie,
			&felt.Address{},
			&path,
			&hash,
			node.IsLeaf(),
			node.Blob(),
		)
		if err != nil {
			return err
		}
		d.cleanCache.putNode(&path, &hash, node.Blob())
	}

	for key, node := range d.dirtyCache.contractNodes {
		path, hash, err := decodeNodeKey([]byte(key))
		if err != nil {
			return err
		}
		err = trieutils.WriteNodeByHash(
			batch,
			db.ContractTrieContract,
			&felt.Address{},
			&path,
			&hash,
			node.IsLeaf(),
			node.Blob(),
		)
		if err != nil {
			return err
		}
		d.cleanCache.putNode(&path, &hash, node.Blob())
	}

	for owner, nodes := range d.dirtyCache.contractStorageNodes {
		for key, node := range nodes {
			path, hash, err := decodeNodeKey([]byte(key))
			if err != nil {
				return err
			}
			err = trieutils.WriteNodeByHash(
				batch,
				db.ContractTrieStorage,
				&owner,
				&path,
				&hash,
				node.IsLeaf(),
				node.Blob(),
			)
			if err != nil {
				return err
			}
			d.cleanCache.putNode(&path, &hash, node.Blob())
		}
	}

	if err := batch.Write(); err != nil {
		return err
	}

	d.dirtyCache.reset()

	d.log.Debugw("Flushed dirty cache to disk",
		"nodes", nodes-d.dirtyCache.len(),
		"duration", time.Since(startTime),
		"liveNodes", d.dirtyCache.len(),
		"liveSize", d.dirtyCache.size,
	)
	return nil
}

func (d *Database) Update(
	root,
	parent *felt.Felt,
	blockNum uint64,
	mergedClassNodes *trienode.MergeNodeSet,
	mergedContractNodes *trienode.MergeNodeSet,
) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	var classNodes map[trieutils.Path]trienode.TrieNode
	var contractNodes map[trieutils.Path]trienode.TrieNode
	var contractStorageNodes map[felt.Address]map[trieutils.Path]trienode.TrieNode

	if mergedClassNodes != nil {
		classNodes, _ = mergedClassNodes.Flatten()
	} else {
		classNodes = make(map[trieutils.Path]trienode.TrieNode)
	}

	if mergedContractNodes != nil {
		contractNodes, contractStorageNodes = mergedContractNodes.Flatten()
	} else {
		contractNodes = make(map[trieutils.Path]trienode.TrieNode)
		contractStorageNodes = make(map[felt.Address]map[trieutils.Path]trienode.TrieNode)
	}

	for path, node := range classNodes {
		if _, ok := node.(*trienode.DeletedNode); ok {
			continue // Since the hashdb is used for archive node only, there is no need to remove nodes
		} else {
			nodeHash := node.Hash()
			d.insert(&felt.Address{}, &path, &nodeHash, true, node)
		}
	}

	for path, node := range contractNodes {
		if _, ok := node.(*trienode.DeletedNode); ok {
			continue
		} else {
			nodeHash := node.Hash()
			d.insert(&felt.Address{}, &path, &nodeHash, false, node)
		}
	}

	for owner, nodes := range contractStorageNodes {
		for path, node := range nodes {
			if _, ok := node.(*trienode.DeletedNode); ok {
				continue
			} else {
				nodeHash := node.Hash()
				d.insert(&owner, &path, &nodeHash, false, node)
			}
		}
	}
	return nil
}

type reader struct {
	id trieutils.TrieID
	d  *Database
}

func (r *reader) Node(
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Felt,
	isLeaf bool,
) ([]byte, error) {
	return r.d.readNode(r.id.Bucket(), owner, path, hash, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{d: d, id: id}, nil
}

func (d *Database) Close() error {
	return nil
}

// TODO(MaksymMalicki): this is a mechanism to detect the node crash, checks if the trie roots associated
// with the state commitment are present in the db, if not, the lost data needs to be recovered
// This will be integrated during the state refactor integration, if there is a node crash,
// the chain needs to be reverted to the last state commitment with the trie roots present in the db
func (d *Database) GetTrieRootNodes(classRootHash, contractRootHash *felt.Felt) (trienode.Node, trienode.Node, error) {
	const contractClassTrieHeight = 251

	classRootBlob, err := trieutils.GetNodeByHash(
		d.disk,
		db.ClassTrie,
		&felt.Address{},
		&trieutils.Path{},
		classRootHash,
		false,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("class root node not found: %w", err)
	}
	if classRootBlob == nil {
		return nil, nil, fmt.Errorf("class root node not found")
	}

	contractRootBlob, err := trieutils.GetNodeByHash(
		d.disk,
		db.ContractTrieContract,
		&felt.Address{},
		&trieutils.Path{},
		contractRootHash,
		false,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("contract root node not found: %w", err)
	}
	if contractRootBlob == nil {
		return nil, nil, fmt.Errorf("contract root node not found")
	}

	classRootNode, err := trienode.DecodeNode(classRootBlob, classRootHash, 0, contractClassTrieHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode class root node: %w", err)
	}

	contractRootNode, err := trienode.DecodeNode(contractRootBlob, contractRootHash, 0, contractClassTrieHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode contract root node: %w", err)
	}

	return classRootNode, contractRootNode, nil
}

func (d *Database) Scheme() database.TrieDBScheme {
	return database.HashScheme
}
