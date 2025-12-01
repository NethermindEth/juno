package pathdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var _ layer = (*diskLayer)(nil)

// Allows access to the underlying database.
// Nodes are buffered in memory and when the buffer size reaches a certain threshold,
// the nodes are flushed to the database.
type diskLayer struct {
	root    felt.Felt // The corresponding state commitment
	id      uint64
	db      *Database
	cleans  *cleanCache // Clean nodes that are already written in the db
	dirties *buffer     // Modified nodes buffered in memory until flushed to disk
	stale   bool
	lock    sync.RWMutex
}

func newDiskLayer(root *felt.Felt, id uint64, db *Database, cache *cleanCache, buffer *buffer) *diskLayer {
	if cache == nil {
		newCleanCache := newCleanCache(db.config.CleanCacheSize)
		cache = &newCleanCache
	}
	return &diskLayer{
		root:    *root,
		id:      id,
		db:      db,
		cleans:  cache,
		dirties: buffer,
		stale:   false,
	}
}

func (dl *diskLayer) parentLayer() layer {
	return nil
}

func (dl *diskLayer) rootHash() *felt.Felt {
	return &dl.root
}

func (dl *diskLayer) stateID() uint64 {
	return dl.id
}

func (dl *diskLayer) isStale() bool {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	return dl.stale
}

func (dl *diskLayer) node(
	id trieutils.TrieID, owner *felt.Address, path *trieutils.Path, isLeaf bool,
) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	if dl.stale {
		return nil, ErrDiskLayerStale
	}

	// Read from dirty buffer first
	isClass := id.Type() == trieutils.Class
	n, ok := dl.dirties.node(owner, path, isClass)
	if ok {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			return nil, db.ErrKeyNotFound
		}
		return n.Blob(), nil
	}

	// If not found in dirty buffer, read from clean cache
	blob := dl.cleans.getNode(owner, path, isClass)
	if blob != nil {
		return blob, nil
	}

	// Finally, read from disk
	blob, err := trieutils.GetNodeByPath(dl.db.disk, id.Bucket(), owner, path, isLeaf)
	if err != nil {
		return nil, err
	}

	dl.cleans.putNode(owner, path, isClass, blob)
	return blob, nil
}

func (dl *diskLayer) update(root *felt.Felt, id, block uint64, nodes *nodeSet) diffLayer {
	return newDiffLayer(dl, root, id, block, nodes)
}

// commit merges the changes from the bottom diff layer into this disk layer.
// If force is true or the buffer is full, changes are flushed to disk.
// Returns a new disk layer with the updated state.
func (dl *diskLayer) commit(bottom *diffLayer, force bool) (*diskLayer, error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	dl.stale = true

	if dl.id == 0 {
		err := trieutils.WriteStateID(dl.db.disk, &dl.root, 0)
		if err != nil {
			return nil, err
		}
	}
	bottomRootHash := bottom.rootHash()
	err := trieutils.WriteStateID(dl.db.disk, bottomRootHash, bottom.stateID())
	if err != nil {
		return nil, err
	}

	combined := dl.dirties.commit(bottom.nodes)
	if force || combined.isFull() {
		if err := combined.flush(dl.db.disk, dl.cleans, bottom.stateID()); err != nil {
			return nil, err
		}
	}
	bottomRootHash = bottom.rootHash()
	newDl := newDiskLayer(bottomRootHash, bottom.stateID(), dl.db, dl.cleans, combined)
	return newDl, nil
}

func (dl *diskLayer) resetCache() {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	if dl.stale {
		return
	}

	if dl.cleans != nil {
		dl.cleans.cache.Reset()
	}
}
