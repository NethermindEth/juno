package pathdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ layer = (*diskLayer)(nil)

type diskLayer struct {
	root    felt.Felt // The corresponding state commitment
	id      uint64
	db      *Database
	cleans  *cleanCache // Clean nodes that are already persisted in the db
	dirties *buffer     // Dirty nodes for writes later
	stale   bool
	lock    sync.RWMutex
}

func newDiskLayer(root felt.Felt, id uint64, db *Database, cache *cleanCache, buffer *buffer) *diskLayer {
	if cache == nil {
		cache = newCleanCache(db.config.CleanCacheSize)
	}
	return &diskLayer{
		root:    root,
		id:      id,
		db:      db,
		cleans:  cache,
		dirties: buffer,
	}
}

func (dl *diskLayer) parentLayer() layer {
	return nil
}

func (dl *diskLayer) rootHash() felt.Felt {
	return dl.root
}

func (dl *diskLayer) stateID() uint64 {
	return dl.id
}

func (dl *diskLayer) isStale() bool {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	return dl.stale
}

func (dl *diskLayer) node(id trieutils.TrieID, owner felt.Felt, path trieutils.Path, isLeaf bool) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	if dl.stale {
		return nil, ErrDiskLayerStale
	}

	// Read from dirty buffer first
	isClass := id.Type() == trieutils.Class
	n, ok := dl.dirties.node(owner, path, isClass)
	if ok {
		return n.Blob(), nil
	}

	// If not found in dirty buffer, read from clean cache
	blob := dl.cleans.getNode(owner, path, isClass)
	if blob != nil {
		return blob, nil
	}

	// Finally, read from disk
	blob, err := trieutils.GetNodeByPath(dl.db.disk, id.Bucket(), owner, path, isClass)
	if err != nil {
		return nil, err
	}

	dl.cleans.putNode(owner, path, isClass, blob)
	return blob, nil
}

func (dl *diskLayer) update(root felt.Felt, id, block uint64, nodes *nodeSet) *diffLayer {
	return newDiffLayer(dl, root, id, block, nodes)
}

// TODO(weiihann): deal with persisted state id later
func (dl *diskLayer) commit(bottom *diffLayer, force bool) (*diskLayer, error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	dl.stale = true

	if dl.id == 0 {
		if err := trieutils.WriteStateID(dl.db.disk, dl.root, 0); err != nil {
			return nil, err
		}
	}
	if err := trieutils.WriteStateID(dl.db.disk, bottom.rootHash(), bottom.stateID()); err != nil {
		return nil, err
	}

	combined := dl.dirties.commit(bottom.nodes)
	if force || combined.isFull() {
		if err := combined.flush(dl.db.disk, dl.cleans, bottom.stateID()); err != nil {
			return nil, err
		}
	}
	newDl := newDiskLayer(bottom.rootHash(), bottom.stateID(), dl.db, dl.cleans, combined)
	return newDl, nil
}
