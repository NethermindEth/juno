package pathdb

import (
	"errors"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type layer interface {
	node(id trieutils.TrieID, owner felt.Felt, path trieutils.Path, isLeaf bool) ([]byte, error)
	update(root felt.Felt, id, block uint64, nodes *nodeSet) *diffLayer
	journal() error
	rootHash() felt.Felt
	stateID() uint64
}

type layerTree struct {
	layers map[felt.Felt]layer
	lock   sync.RWMutex
}

func (l *layerTree) get(root felt.Felt) layer {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.layers[root]
}

func (l *layerTree) add(root, parentRoot felt.Felt, block uint64, mergeClassNodes, mergeContractNodes *trienode.MergeNodeSet) error {
	if root == parentRoot {
		return errors.New("cannot have cycled layer")
	}

	parent := l.get(parentRoot)
	if parent == nil {
		return fmt.Errorf("parent layer %v not found", parentRoot)
	}

	classNodes, _ := mergeClassNodes.Flatten()
	contractNodes, contractStorageNodes := mergeContractNodes.Flatten()

	newLayer := parent.update(root, parent.stateID()+1, block, newNodeSet(classNodes, contractNodes, contractStorageNodes))

	l.lock.Lock()
	l.layers[root] = newLayer
	l.lock.Unlock()

	return nil
}
