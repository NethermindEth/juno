package pathdb

import (
	"errors"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var _ layer = (*diffLayer)(nil)

// Represents an in-memory layer which contains the diff nodes for a specific state root hash
type diffLayer struct {
	root  felt.StateRootHash // State root hash where this diff layer is applied
	id    uint64             // Corresponding state id
	block uint64             // Associated block number
	nodes *nodeSet           // Cached trie nodes

	parent layer // Parent layer
	lock   sync.RWMutex
}

func newDiffLayer(
	parent layer,
	root *felt.StateRootHash,
	id, block uint64,
	nodes *nodeSet,
) diffLayer {
	return diffLayer{
		root:   *root,
		id:     id,
		block:  block,
		nodes:  nodes,
		parent: parent,
	}
}

func (dl *diffLayer) node(
	id trieutils.TrieID,
	owner *felt.Address,
	path *trieutils.Path,
	isLeaf bool,
) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	isClass := id.Type() == trieutils.Class
	n, ok := dl.nodes.node(owner, path, isClass)
	if ok {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			return nil, db.ErrKeyNotFound
		}
		return n.Blob(), nil
	}

	return dl.parent.node(id, owner, path, isLeaf)
}

func (dl *diffLayer) rootHash() *felt.StateRootHash {
	return &dl.root
}

func (dl *diffLayer) stateID() uint64 {
	return dl.id
}

func (dl *diffLayer) update(root *felt.StateRootHash, id, block uint64, nodes *nodeSet) diffLayer {
	return newDiffLayer(dl, root, id, block, nodes)
}

func (dl *diffLayer) persist(force bool) (layer, error) {
	if parent, ok := dl.parentLayer().(*diffLayer); ok {
		dl.lock.Lock()

		result, err := parent.persist(force)
		if err != nil {
			dl.lock.Unlock()
			return nil, err
		}
		dl.parent = result
		dl.lock.Unlock()
	}

	disk, ok := dl.parentLayer().(*diskLayer)
	if !ok {
		return nil, errors.New("parent layer is not a disk layer")
	}

	return disk.commit(dl, force)
}

func (dl *diffLayer) parentLayer() layer {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.parent
}
