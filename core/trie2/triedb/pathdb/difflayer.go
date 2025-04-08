package pathdb

import (
	"errors"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var _ layer = (*diffLayer)(nil)

type diffLayer struct {
	root  felt.Felt // State root hash where this diff layer is applied
	id    uint64    // Corresponding state id
	block uint64    // Associated block number
	nodes *nodeSet  // Cached trie nodes

	parent layer // Parent layer
	lock   sync.RWMutex
}

func newDiffLayer(parent layer, root felt.Felt, id, block uint64, nodes *nodeSet) *diffLayer {
	return &diffLayer{
		root:   root,
		id:     id,
		block:  block,
		nodes:  nodes,
		parent: parent,
	}
}

func (dl *diffLayer) node(id trieutils.TrieID, owner felt.Felt, path trieutils.Path, isLeaf bool) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	isClass := id.Type() == trieutils.Class
	n, ok := dl.nodes.node(owner, path, isClass)
	if ok {
		return n.Blob(), nil
	}

	return dl.parent.node(id, owner, path, isClass)
}

func (dl *diffLayer) rootHash() felt.Felt {
	return dl.root
}

func (dl *diffLayer) stateID() uint64 {
	return dl.id
}

func (dl *diffLayer) update(root felt.Felt, id, block uint64, nodes *nodeSet) *diffLayer {
	return newDiffLayer(dl.parent, root, id, block, nodes)
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

	return diffToDisk(dl, force)
}

func (dl *diffLayer) parentLayer() layer {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.parent
}

func diffToDisk(dl *diffLayer, force bool) (layer, error) {
	disk, ok := dl.parentLayer().(*diskLayer)
	if !ok {
		return nil, errors.New("parent layer is not a disk layer")
	}

	return disk.commit(dl, force)
}
