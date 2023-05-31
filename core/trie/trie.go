// Package trie implements a dense Merkle Patricia Trie. See the documentation on [Trie] for details.
package trie

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
)

// Storage is the Persistent storage for the [Trie]
type Storage interface {
	Put(key *bitset.BitSet, value *Node) error
	Get(key *bitset.BitSet) (*Node, error)
	Delete(key *bitset.BitSet) error
}

type hashFunc func(*felt.Felt, *felt.Felt) *felt.Felt

// Trie is a dense Merkle Patricia Trie (i.e., all internal nodes have two children).
//
// This implementation allows for a "flat" storage by keying nodes on their path rather than
// their hash, resulting in O(1) accesses and O(log n) insertions.
//
// The state trie [specification] describes a sparse Merkle Trie.
// Note that this dense implementation results in an equivalent commitment.
//
// Terminology:
//   - path: represents the path as defined in the specification. Together with len,
//     represents a relative path from the current node to the node's nearest non-empty child.
//   - len: represents the len as defined in the specification. The number of bits to take
//     from the fixed-length path to reach the nearest non-empty child.
//   - key: represents the storage key for trie [Node]s. It is the full path to the node from the
//     root.
//
// [specification]: https://docs.starknet.io/documentation/develop/State/starknet-state/
type Trie struct {
	height  uint
	rootKey *bitset.BitSet
	maxKey  *felt.Felt
	storage Storage
	hash    hashFunc

	dirtyNodes []*bitset.BitSet
}

type NewTrieFunc func(Storage, uint, *bitset.BitSet) (*Trie, error)

func NewTriePedersen(storage Storage, height uint, rootKey *bitset.BitSet) (*Trie, error) {
	return newTrie(storage, height, rootKey, crypto.Pedersen)
}

func NewTriePoseidon(storage Storage, height uint, rootKey *bitset.BitSet) (*Trie, error) {
	return newTrie(storage, height, rootKey, crypto.Poseidon)
}

func newTrie(storage Storage, height uint, rootKey *bitset.BitSet, hash hashFunc) (*Trie, error) {
	if height > felt.Bits {
		return nil, fmt.Errorf("max trie height is %d, got: %d", felt.Bits, height)
	}

	// maxKey is 2^height - 1
	maxKey := new(felt.Felt).Exp(new(felt.Felt).SetUint64(2), new(big.Int).SetUint64(uint64(height)))
	maxKey.Sub(maxKey, new(felt.Felt).SetUint64(1))

	return &Trie{
		storage: storage,
		height:  height,
		rootKey: rootKey,
		maxKey:  maxKey,
		hash:    hash,
	}, nil
}

// RunOnTempTrie creates an in-memory Trie of height `height` and runs `do` on that Trie
func RunOnTempTrie(height uint, do func(*Trie) error) error {
	trie, err := NewTriePedersen(newMemStorage(), height, nil)
	if err != nil {
		return err
	}
	return do(trie)
}

// feltToBitSet Converts a key, given in felt, to a bitset which when followed on a [Trie],
// leads to the corresponding [Node]
func (t *Trie) feltToBitSet(k *felt.Felt) *bitset.BitSet {
	if k == nil {
		return nil
	}

	kBits := k.Bits()
	return bitset.FromWithLength(t.height, kBits[:])
}

// findCommonKey finds the set of common MSB bits in two key bitsets.
func findCommonKey(longerKey, shorterKey *bitset.BitSet) (*bitset.BitSet, bool) {
	divergentBit := findDivergentBit(longerKey, shorterKey)
	commonKey := shorterKey.Clone()
	for i := uint(0); i < shorterKey.Len()-divergentBit+1; i++ {
		commonKey.DeleteAt(0)
	}
	return commonKey, divergentBit == shorterKey.Len()+1
}

func findDivergentBit(longerKey, shorterKey *bitset.BitSet) uint {
	divergentBit := uint(0)
	// todo: use NextSetMany for performance
	for divergentBit <= shorterKey.Len() &&
		longerKey.Test(longerKey.Len()-divergentBit) == shorterKey.Test(shorterKey.Len()-divergentBit) {
		divergentBit++
	}
	return divergentBit
}

func isSubset(longerKey, shorterKey *bitset.BitSet) bool {
	divergentBit := findDivergentBit(longerKey, shorterKey)
	return divergentBit == shorterKey.Len()+1
}

// path returns the path as mentioned in the [specification] for commitment calculations.
// path is suffix of key that diverges from parentKey. For example,
// for a key 0b1011 and parentKey 0b10, this function would return the path object of 0b0.
//
// [specification]: https://docs.starknet.io/documentation/develop/State/starknet-state/
func path(key, parentKey *bitset.BitSet) *bitset.BitSet {
	path := key.Clone()
	// drop parent key, and one more MSB since left/right relation already encodes that information
	if parentKey != nil {
		path.Shrink(path.Len() - parentKey.Len() - 1)
		path.DeleteAt(path.Len() - 1)
	}
	return path
}

// storageNode is the on-disk representation of a [Node],
// where key is the storage key and node is the value.
type storageNode struct {
	key  *bitset.BitSet
	node *Node
}

// nodesFromRoot enumerates the set of [Node] objects that need to be traversed from the root
// of the Trie to the node which is given by the key.
// The [storageNode]s are returned in descending order beginning with the root.
func (t *Trie) nodesFromRoot(key *bitset.BitSet) ([]storageNode, error) {
	var nodes []storageNode
	cur := t.rootKey
	for cur != nil {
		node, err := t.storage.Get(cur)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, storageNode{
			key:  cur,
			node: node,
		})

		subset := isSubset(key, cur)
		if cur.Len() >= key.Len() || !subset {
			return nodes, nil
		}

		if key.Test(key.Len() - cur.Len() - 1) {
			cur = node.Right
		} else {
			cur = node.Left
		}
	}

	return nodes, nil
}

// Get the corresponding `value` for a `key`
func (t *Trie) Get(key *felt.Felt) (*felt.Felt, error) {
	value, err := t.storage.Get(t.feltToBitSet(key))
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return &felt.Zero, nil
		}
		return nil, err
	}
	return value.Value, nil
}

// Put updates the corresponding `value` for a `key`
//
//nolint:gocyclo
func (t *Trie) Put(key, value *felt.Felt) (*felt.Felt, error) {
	if key.Cmp(t.maxKey) > 0 {
		return nil, fmt.Errorf("key %s exceeds trie height %d", key, t.height)
	}

	old := new(felt.Felt)
	nodeKey := t.feltToBitSet(key)
	node := &Node{
		Value: value,
	}

	nodes, err := t.nodesFromRoot(nodeKey)
	if err != nil {
		return nil, err
	}

	// empty trie, make new value root
	if len(nodes) == 0 {
		if value.IsZero() {
			return nil, nil // no-op
		}

		if err = t.storage.Put(nodeKey, node); err != nil {
			return nil, err
		}
		t.rootKey = nodeKey
		return old, nil
	} else {
		// Replace if key already exist
		sibling := nodes[len(nodes)-1]
		if nodeKey.Equal(sibling.key) {
			old = sibling.node.Value // record old value to return to caller
			if value.IsZero() {
				if err = t.deleteLast(nodes); err != nil {
					return nil, err
				}
				return old, nil
			}

			if err = t.storage.Put(nodeKey, node); err != nil {
				return nil, err
			}
			t.dirtyNodes = append(t.dirtyNodes, nodeKey)
			return old, nil
		} else if value.IsZero() {
			// trying to insert 0 to a key that does not exist
			return nil, nil // no-op
		}

		commonKey, _ := findCommonKey(nodeKey, sibling.key)
		newParent := &Node{}
		var leftChild, rightChild *Node
		if nodeKey.Test(nodeKey.Len() - commonKey.Len() - 1) {
			newParent.Left, newParent.Right = sibling.key, nodeKey
			leftChild, rightChild = sibling.node, node
		} else {
			newParent.Left, newParent.Right = nodeKey, sibling.key
			leftChild, rightChild = node, sibling.node
		}

		leftPath := path(newParent.Left, commonKey)
		rightPath := path(newParent.Right, commonKey)

		newParent.Value = t.hash(leftChild.Hash(leftPath, t.hash), rightChild.Hash(rightPath, t.hash))
		if err = t.storage.Put(commonKey, newParent); err != nil {
			return nil, err
		}

		if len(nodes) > 1 { // sibling has a parent
			siblingParent := nodes[len(nodes)-2]

			// replace the link to our sibling with the new parent
			if siblingParent.node.Left.Equal(sibling.key) {
				siblingParent.node.Left = commonKey
			} else {
				siblingParent.node.Right = commonKey
			}

			if err = t.storage.Put(siblingParent.key, siblingParent.node); err != nil {
				return nil, err
			}
			t.dirtyNodes = append(t.dirtyNodes, commonKey)
		} else {
			t.rootKey = commonKey
		}

		if err = t.storage.Put(nodeKey, node); err != nil {
			return nil, err
		}
		return old, nil
	}
}

func (t *Trie) updateValueIfDirty(key *bitset.BitSet) (*Node, error) {
	node, err := t.storage.Get(key)
	if err != nil {
		return nil, err
	}

	// leaf node
	if key.Len() == t.height {
		return node, nil
	}

	shouldUpdate := false
	for _, dirtyNode := range t.dirtyNodes {
		if key.Len() < dirtyNode.Len() {
			shouldUpdate = isSubset(dirtyNode, key)
			if shouldUpdate {
				break
			}
		}
	}

	if !shouldUpdate {
		return node, nil
	}

	leftChild, err := t.updateValueIfDirty(node.Left)
	if err != nil {
		return nil, err
	}
	rightChild, err := t.updateValueIfDirty(node.Right)
	if err != nil {
		return nil, err
	}

	leftPath := path(node.Left, key)
	rightPath := path(node.Right, key)

	node.Value = t.hash(leftChild.Hash(leftPath, t.hash), rightChild.Hash(rightPath, t.hash))

	if err = t.storage.Put(key, node); err != nil {
		return nil, err
	}
	return node, nil
}

// deleteLast deletes the last node in the given list
func (t *Trie) deleteLast(nodes []storageNode) error {
	last := nodes[len(nodes)-1]
	if err := t.storage.Delete(last.key); err != nil {
		return err
	}

	if len(nodes) == 1 { // deleted node was root
		t.rootKey = nil
		return nil
	}

	// parent now has only a single child, so delete
	parent := nodes[len(nodes)-2]
	if err := t.storage.Delete(parent.key); err != nil {
		return err
	}

	var siblingKey *bitset.BitSet
	if parent.node.Left.Equal(last.key) {
		siblingKey = parent.node.Right
	} else {
		siblingKey = parent.node.Left
	}

	if len(nodes) == 2 { // sibling should become root
		t.rootKey = siblingKey
		return nil
	}
	// sibling should link to grandparent (len(affectedNodes) > 2)
	grandParent := &nodes[len(nodes)-3]
	// replace link to parent with a link to sibling
	if grandParent.node.Left.Equal(parent.key) {
		grandParent.node.Left = siblingKey
	} else {
		grandParent.node.Right = siblingKey
	}

	if err := t.storage.Put(grandParent.key, grandParent.node); err != nil {
		return err
	}
	t.dirtyNodes = append(t.dirtyNodes, siblingKey)
	return nil
}

// Root returns the commitment of a [Trie]
func (t *Trie) Root() (*felt.Felt, error) {
	if t.rootKey == nil {
		return new(felt.Felt), nil
	}

	root, err := t.updateValueIfDirty(t.rootKey)
	if err != nil {
		return nil, err
	}
	t.dirtyNodes = nil

	path := path(t.rootKey, nil)
	return root.Hash(path, t.hash), nil
}

// Commit forces root calculation
func (t *Trie) Commit() error {
	_, err := t.Root()
	return err
}

// RootKey returns db key of the [Trie] root node
func (t *Trie) RootKey() *bitset.BitSet {
	return t.rootKey
}

func (t *Trie) Dump() {
	t.dump(0, nil)
}

// Try to print a [Trie] in a somewhat human-readable form
/*
Todo: create more meaningful representation of trie. In the current format string, storage is being
printed but the value that is printed is the bitset of the trie node this is different from the
storage of the trie. Also, consider renaming the function name to something other than dump.

The following can be printed:
- key (which represents the storage key)
- path (as specified in the documentation)
- len (as specified in the documentation)
- bottom (as specified in the documentation)

The spacing to represent the levels of the trie can remain the same.
*/
func (t *Trie) dump(level int, parentP *bitset.BitSet) {
	if t.rootKey == nil {
		fmt.Printf("%sEMPTY\n", strings.Repeat("\t", level))
		return
	}

	root, err := t.storage.Get(t.rootKey)
	path := path(t.rootKey, parentP)
	fmt.Printf("%sstorage : \"%s\" %d spec: \"%s\" %d bottom: \"%s\" \n",
		strings.Repeat("\t", level),
		t.rootKey.String(),
		t.rootKey.Len(),
		path.String(),
		path.Len(),
		root.Value.String(),
	)
	if err != nil {
		return
	}
	(&Trie{
		rootKey: root.Left,
		storage: t.storage,
	}).dump(level+1, t.rootKey)
	(&Trie{
		rootKey: root.Right,
		storage: t.storage,
	}).dump(level+1, t.rootKey)
}
