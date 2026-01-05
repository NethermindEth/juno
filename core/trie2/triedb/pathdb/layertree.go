package pathdb

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

// Represents a layer of the layer tree
type layer interface {
	// Returns the encoded node bytes for a given trie id, owner, path and isLeaf flag
	node(id trieutils.TrieID, owner *felt.Address, path *trieutils.Path, isLeaf bool) ([]byte, error)
	// Updates the layer with a new root hash, state id and block number
	update(root *felt.StateRootHash, id, block uint64, nodes *nodeSet) diffLayer
	// Writes the journal to the given writer
	journal(w io.Writer) error
	// Returns the root hash of the layer
	rootHash() *felt.StateRootHash
	// Returns the state id of the layer
	stateID() uint64
	// Returns the parent layer of the current layer
	parentLayer() layer
}

// Represents a layer tree which contains multiple in-memory diff layers and a single disk layer.
// The disk layer must be at the bottom of the tree and there can only be one.
type layerTree struct {
	layers map[felt.StateRootHash]layer
	lock   sync.RWMutex
}

func newLayerTree(head layer) *layerTree {
	tree := new(layerTree)
	tree.reset(head)
	return tree
}

// Returns the layer for a given root hash
func (tree *layerTree) get(root *felt.StateRootHash) layer {
	tree.lock.RLock()
	defer tree.lock.RUnlock()

	return tree.layers[*root]
}

// Adds a new layer to the layer tree
func (tree *layerTree) add(
	root,
	parentRoot *felt.StateRootHash,
	block uint64,
	mergeClassNodes,
	mergeContractNodes *trienode.MergeNodeSet,
) error {
	if root == parentRoot {
		return errors.New("cyclic layer detected, root and parent root cannot be the same")
	}

	parent := tree.get(parentRoot)
	if parent == nil {
		return fmt.Errorf("parent layer %v not found", parentRoot)
	}

	var classNodes classNodesMap
	var contractNodes contractNodesMap
	var contractStorageNodes contractStorageNodesMap

	if mergeClassNodes != nil {
		classNodes, _ = mergeClassNodes.Flatten()
	}
	if mergeContractNodes != nil {
		contractNodes, contractStorageNodes = mergeContractNodes.Flatten()
	}

	newLayer := parent.update(root, parent.stateID()+1, block, newNodeSet(classNodes, contractNodes, contractStorageNodes))

	tree.lock.Lock()
	tree.layers[*root] = &newLayer
	tree.lock.Unlock()

	return nil
}

// Traverses the layer tree and check if the number of diffs exceeds the given number of layers.
// If it does, the bottom-most layer will be merged to the disk layer.
//
//nolint:gocyclo
func (tree *layerTree) cap(root *felt.StateRootHash, layers int) error {
	l := tree.get(root)
	if l == nil {
		return fmt.Errorf("layer %v not found", root)
	}

	diff, ok := l.(*diffLayer)
	if !ok {
		return nil
	}

	tree.lock.Lock()
	defer tree.lock.Unlock()

	if layers == 0 {
		base, err := diff.persist(true)
		if err != nil {
			return err
		}

		rootHash := base.rootHash()
		tree.layers = map[felt.StateRootHash]layer{*rootHash: base}
		return nil
	}

	// Traverse the layer tree until we reach the desired number of layers
	// If we reach the disk layer, it means that the layer tree is still shallow
	// and can allow for further growing. So we return early.
	for range layers - 1 {
		if parent, ok := diff.parentLayer().(*diffLayer); ok {
			diff = parent
		} else {
			return nil
		}
	}

	// At this point, the layer tree is full, so we need to flatten.
	switch parent := diff.parentLayer().(type) {
	case *diskLayer:
		return nil
	case *diffLayer:
		diff.lock.Lock()

		base, err := parent.persist(false)
		if err != nil {
			diff.lock.Unlock()
			return err
		}
		baseRootHash := base.rootHash()
		tree.layers[*baseRootHash] = base
		diff.parent = base
		diff.lock.Unlock()
	default:
		return fmt.Errorf("unexpected parent layer type: %T", parent)
	}

	// Remove layers that are stale
	children := make(map[felt.StateRootHash][]felt.StateRootHash)
	for root, layer := range tree.layers {
		if dl, ok := layer.(*diffLayer); ok {
			parent := dl.parentLayer().rootHash()
			children[*parent] = append(children[*parent], root)
		}
	}

	var removeLinks func(root *felt.StateRootHash)
	removeLinks = func(root *felt.StateRootHash) {
		delete(tree.layers, *root)
		for _, child := range children[*root] {
			removeLinks(&child)
		}
		delete(children, *root)
	}

	for root, layers := range tree.layers {
		if dl, ok := layers.(*diskLayer); ok && dl.isStale() {
			removeLinks(&root)
		}
	}

	return nil
}

func (tree *layerTree) reset(head layer) {
	tree.lock.Lock()
	defer tree.lock.Unlock()

	layers := make(map[felt.StateRootHash]layer)
	for head != nil {
		headRootHash := head.rootHash()
		layers[*headRootHash] = head
		head = head.parentLayer()
	}
	tree.layers = layers
}

func (tree *layerTree) len() int {
	tree.lock.RLock()
	defer tree.lock.RUnlock()

	return len(tree.layers)
}

func (tree *layerTree) diskLayer() *diskLayer {
	tree.lock.RLock()
	defer tree.lock.RUnlock()

	if len(tree.layers) == 0 {
		return nil
	}

	var current layer
	for _, layer := range tree.layers {
		current = layer
		break
	}

	for current.parentLayer() != nil {
		current = current.parentLayer()
	}
	return current.(*diskLayer)
}
