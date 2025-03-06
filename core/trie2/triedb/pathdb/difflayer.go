package pathdb

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

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
		block:  block,
		nodes:  nodes,
		parent: parent,
	}
}

func (dl *diffLayer) node(owner felt.Felt, path trieutils.Path, isClass bool) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	n, ok := dl.nodes.node(owner, path, isClass)
	if ok {
		return n.Blob(), nil
	}

	return dl.parent.node(owner, path, isClass)
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

func (dl *diffLayer) persist() (layer, error) {
	if parent, ok := dl.parentLayer().(*diffLayer); ok {
		dl.lock.Lock()

		result, err := parent.persist()
		if err != nil {
			dl.lock.Unlock()
			return nil, err
		}
		dl.parent = result
		dl.lock.Unlock()
	}

	return diffToDisk(dl)
}

func (dl *diffLayer) parentLayer() layer {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.parent
}

func (dl *diffLayer) journal() error {
	panic("TODO(weiihann): implement me")
}

func diffToDisk(dl *diffLayer) (layer, error) {
	panic("TODO(weiihann): implement me")
}
