package hashdb

import (
	"errors"
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

var ErrCallEmptyDatabase = errors.New("call to empty database")

type Config struct {
	DirtyCacheSize int
	CleanCacheSize int

	DirtyCacheType CacheType
	CleanCacheType CacheType

	//TODO: This is a temporary solution, need to find a better way to handle this
	maxPathLen uint8
}

var DefaultConfig = &Config{
	DirtyCacheSize: 1024 * 1024 * 64,
	CleanCacheSize: 1024 * 1024 * 64,

	DirtyCacheType: CacheTypeLRU,
	CleanCacheType: CacheTypeFastCache,

	maxPathLen: contractClassTrieHeight,
}

type Database struct {
	disk   db.KeyValueStore
	config *Config

	cleanCache CleanCache
	dirtyCache DirtyCache
	rootsCache CleanCache

	dirtyCacheSize int

	log  utils.SimpleLogger
	lock sync.RWMutex
}

const (
	maxCacheSize            = 100 * 1024 * 1024
	idealBatchSize          = 100 * 1024
	rootsCacheSize          = 100
	flushInterval           = 5 * time.Minute
	storageKeySize          = 75
	contractClassKeySize    = 44
	contractClassTrieHeight = 251
)

func New(disk db.KeyValueStore, config *Config) *Database {
	if config == nil {
		config = DefaultConfig
	}
	return &Database{
		disk:       disk,
		config:     config,
		cleanCache: NewCleanCache(config.CleanCacheType, config.CleanCacheSize),
		dirtyCache: NewDirtyCache(config.DirtyCacheType, config.DirtyCacheSize),
		rootsCache: NewCleanCache(config.CleanCacheType, rootsCacheSize),
		log:        utils.NewNopZapLogger(),
	}
}

func (d *Database) insert(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	_, found := d.dirtyCache.Get(key)
	if found {
		return
	}
	d.dirtyCache.Set(key, cachedNode{
		blob: blob,
	})
	d.dirtyCacheSize += len(blob)
}

func (d *Database) node(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	if d.cleanCache != nil {
		blob, found := d.cleanCache.Get(key)
		if found {
			return blob, nil
		}
	}

	if d.dirtyCache != nil {
		node, found := d.dirtyCache.Get(key)
		if found {
			return node.blob, nil
		}
	}

	var blob []byte
	err := d.disk.Get(key, func(value []byte) error {
		blob = value
		return nil
	})
	if err != nil {
		return nil, err
	}

	if d.cleanCache != nil {
		d.cleanCache.Set(key, blob)
	}

	return blob, nil
}

func (d *Database) remove(bucket db.Bucket, owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) {
	key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
	d.dirtyCache.Remove(key)
	d.dirtyCacheSize -= len(blob)
}

func (d *Database) NewIterator(id trieutils.TrieID) (db.Iterator, error) {
	key := id.Bucket().Key()
	owner := id.Owner()
	if !owner.Equal(&felt.Zero) {
		oBytes := owner.Bytes()
		key = append(key, oBytes[:]...)
	}

	return d.disk.NewIterator(key, true)
}

func (d *Database) Cap(limit uint64) error {
	batch := d.disk.NewBatch()
	key, node, cacheNotEmpty := d.dirtyCache.GetOldest()
	nodes, dirtyCacheSize, startTime := d.dirtyCache.Len(), d.dirtyCacheSize, time.Now()

	for uint64(d.dirtyCacheSize) > limit && cacheNotEmpty {
		if err := batch.Put(key, node.blob); err != nil {
			d.log.Errorw("Failed to write flush list to disk", "error", err)
			return err
		}

		if batch.Size() > idealBatchSize {
			if err := batch.Write(); err != nil {
				d.log.Errorw("Failed to write flush list to disk", "error", err)
				return err
			}
			batch.Reset()
		}

		d.dirtyCacheSize -= len(node.blob)
		ok := d.dirtyCache.RemoveOldest()
		if !ok {
			return fmt.Errorf("oldest element in dirty cache not found")
		}
		key, node, cacheNotEmpty = d.dirtyCache.GetOldest()
	}

	if batch.Size() > 0 {
		if err := batch.Write(); err != nil {
			d.log.Errorw("Failed to write flush list to disk", "error", err)
			return err
		}
	}

	d.log.Debugw("Flushed dirty cache to disk",
		"size", dirtyCacheSize-d.dirtyCacheSize,
		"nodes", nodes-d.dirtyCache.Len(),
		"duration", time.Since(startTime),
		"liveNodes", d.dirtyCache.Len(),
		"liveSize", d.dirtyCacheSize,
	)
	return nil
}

func (d *Database) Commit(stateComm felt.Felt) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	classRoot, contractRoot, err := d.getRootsForStateCommitment(stateComm)
	if err != nil {
		return fmt.Errorf("failed to get roots for state commitment: %w", err)
	}

	if err := d.commitDirtyTrie(*classRoot, db.ClassTrie); err != nil {
		return err
	}

	if err := d.commitDirtyTrie(*contractRoot, db.ContractTrieContract); err != nil {
		return err
	}

	return nil
}

func (d *Database) commitDirtyTrie(root felt.Felt, bucket db.Bucket) error {
	start := time.Now()
	batch := d.disk.NewBatch()

	nodes := d.dirtyCache.Len()
	size := d.dirtyCacheSize

	if err := d.commit(batch, bucket, trieutils.Path{}, root); err != nil {
		d.log.Errorw("Failed to commit trie", "err", err)
		return err
	}

	// Write any remaining batch entries
	if err := batch.Write(); err != nil {
		d.log.Errorw("Failed to write trie to disk", "err", err)
		return err
	}
	batch.Reset()

	// Log statistics
	d.log.Debugw("Persisted trie from memory database",
		"nodes", nodes-d.dirtyCache.Len(),
		"size", size-d.dirtyCacheSize,
		"time", time.Since(start),
		"root", root.String(),
	)

	return nil
}

func (d *Database) commit(batch db.Batch, bucket db.Bucket, path trieutils.Path, rootHash felt.Felt) error {
	// If the node does not exist, it's a previously committed node
	// TODO: This is a temporary solution, need to find a better way to handle this
	isLeaf := path.Len() == d.config.maxPathLen
	key := trieutils.NodeKeyByHash(bucket, felt.Zero, path, rootHash, isLeaf)
	root, found := d.dirtyCache.Get(key)
	if !found {
		return nil
	}
	if !isLeaf {
		children, err := d.getNodeChildren(root.blob)
		if err != nil {
			return fmt.Errorf("failed to get node children: %w", err)
		}
		for i, childKey := range children {
			childPath := new(trieutils.Path).AppendBit(&path, uint8(i))
			if err := d.commit(batch, bucket, *childPath, childKey); err != nil {
				return err
			}
		}
	}

	if err := batch.Put(key, root.blob); err != nil {
		return err
	}

	d.dirtyCache.Remove(key)
	d.cleanCache.Set(key, root.blob)
	d.dirtyCacheSize -= len(root.blob)

	if batch.Size() >= idealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		batch.Reset()
	}

	return nil
}

// TODO: This is temporary solution, have to think of a way to parse nodes without cyclic dependency
// Binary Node: binaryNodeType(1) + HashNode(left) + HashNode(right)
// Edge Node: edgeNodeType(2) + HashNode(child) + Path
// Hash/Value Node: just the felt bytes
func (d *Database) getNodeChildren(blob []byte) ([]felt.Felt, error) {
	if len(blob) == 0 {
		return nil, errors.New("empty blob")
	}

	if len(blob) == felt.Bytes {
		return nil, nil
	}

	var children []felt.Felt
	nodeType := blob[0]
	blob = blob[1:]

	switch nodeType {
	case 1:
		if len(blob) < 2*felt.Bytes {
			return nil, fmt.Errorf("invalid binary node size: %d", len(blob))
		}

		leftHash := new(felt.Felt)
		leftHash.SetBytes(blob[:felt.Bytes])
		if !leftHash.IsZero() {
			children = append(children, *leftHash)
		}

		rightHash := new(felt.Felt)
		rightHash.SetBytes(blob[felt.Bytes : 2*felt.Bytes])
		if !rightHash.IsZero() {
			children = append(children, *rightHash)
		}

	case 2:
		if len(blob) < felt.Bytes {
			return nil, fmt.Errorf("invalid edge node size: %d", len(blob))
		}

		childHash := new(felt.Felt)
		childHash.SetBytes(blob[:felt.Bytes])
		if !childHash.IsZero() {
			children = append(children, *childHash)
		}

	default:
		return nil, fmt.Errorf("unknown node type: %d", nodeType)
	}

	return children, nil
}

func (d *Database) getRootsForStateCommitment(stateComm felt.Felt) (*felt.Felt, *felt.Felt, error) {
	bytes := stateComm.Bytes()
	key := db.StateHashToRoots.Key(bytes[:])

	if d.rootsCache != nil {
		if val, found := d.rootsCache.Get(key); found {
			return decodeRoots(val)
		}
	}

	var rootsData []byte
	err := d.disk.Get(key, func(val []byte) error {
		rootsData = val
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get roots for state hash %v: %w", stateComm, err)
	}

	if d.rootsCache != nil {
		d.rootsCache.Set(key, rootsData)
	}

	return decodeRoots(rootsData)
}

func decodeRoots(val []byte) (*felt.Felt, *felt.Felt, error) {
	if len(val) != 2*felt.Bytes {
		return nil, nil, fmt.Errorf("invalid value length: got %d, want %d", len(val), 2*felt.Bytes)
	}
	var classRoot, contractRoot felt.Felt
	classRoot.SetBytes(val[:felt.Bytes])
	contractRoot.SetBytes(val[felt.Bytes:])
	return &classRoot, &contractRoot, nil
}

func (d *Database) Update(
	root,
	parent felt.Felt,
	blockNum uint64,
	classNodes map[trieutils.Path]trienode.TrieNode,
	contractNodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode,
) {
	for path, node := range classNodes {
		if _, ok := node.(*trienode.DeletedNode); ok {
			d.remove(db.ClassTrie, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf())
		} else {
			d.insert(db.ClassTrie, felt.Zero, path, node.Hash(), node.Blob(), node.IsLeaf())
		}
	}

	for owner, nodes := range contractNodes {
		bucket := db.ContractTrieStorage
		if owner.Equal(&felt.Zero) {
			bucket = db.ContractTrieContract
		}

		for path, node := range nodes {
			if _, ok := node.(*trienode.DeletedNode); ok {
				d.remove(bucket, owner, path, node.Hash(), node.Blob(), node.IsLeaf())
			} else {
				d.insert(bucket, owner, path, node.Hash(), node.Blob(), node.IsLeaf())
			}
		}
	}
}

type reader struct {
	id trieutils.TrieID
	d  *Database
}

func (r *reader) Node(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	return r.d.node(r.id.Bucket(), owner, path, hash, isLeaf)
}

func (d *Database) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	return &reader{d: d, id: id}, nil
}

func (d *Database) Close() error {
	return nil
}
