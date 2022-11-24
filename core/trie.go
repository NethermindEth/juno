package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
)

// Persistent storage backend for [Trie]
type TrieStorage interface {
	Put(key *bitset.BitSet, value *TrieNode) error
	Get(key *bitset.BitSet) (*TrieNode, error)
}

// A [Trie] node
type TrieNode struct {
	value *felt.Felt
	left  *bitset.BitSet
	right *bitset.BitSet
}

// Calculates hash of a [TrieNode]
func (n *TrieNode) Hash(specPath *bitset.BitSet) *felt.Felt {
	if specPath.Len() == 0 {
		return n.value
	}

	pathWords := specPath.Bytes()
	if len(pathWords) > 4 {
		panic("Path too long to fit in Felt")
	}

	var pathBytes [32]byte
	for idx, word := range pathWords {
		startBytes := 24 - (idx * 8)
		binary.BigEndian.PutUint64(pathBytes[startBytes:startBytes+8], word)
	}

	pathFelt := felt.NewFelt(0)
	(&pathFelt).SetBytes(pathBytes[:])

	// https://docs.starknet.io/documentation/develop/State/starknet-state/
	hash, err := crypto.Pedersen(n.value, &pathFelt)
	if err != nil {
		panic("Pedersen failed TrieNode.Hash")
	}

	pathFelt.SetUint64(uint64(specPath.Len()))
	return hash.Add(hash, &pathFelt)
}

// Equality check of 2 [TrieNode] objects
func (n *TrieNode) Equal(other *TrieNode) bool {
	return n.value.Equal(other.value) && n.left.Equal(other.left) && n.right.Equal(n.right)
}

// Serializes a [TrieNode] into a byte array
func (n *TrieNode) MarshalBinary() ([]byte, error) {
	var ret []byte
	valueB := n.value.Bytes()
	ret = append(ret, valueB[:]...)

	if n.left != nil {
		ret = append(ret, 'l')
		leftB, err := n.left.MarshalBinary()
		if err != nil {
			return nil, err
		}
		ret = append(ret, leftB...)
	}

	if n.right != nil {
		ret = append(ret, 'r')
		rightB, err := n.right.MarshalBinary()
		if err != nil {
			return nil, err
		}
		ret = append(ret, rightB...)
	}
	return ret, nil
}

// Deserializes a [TrieNode] from a byte array
func (n *TrieNode) UnmarshalBinary(data []byte) error {
	if len(data) < felt.Bytes {
		return errors.New("Malformed TrieNode bytedata")
	}
	n.value = new(felt.Felt).SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]

	stream := bytes.NewReader(data)
	for stream.Len() > 0 {
		head, err := stream.ReadByte()
		if err != nil {
			return err
		}

		var pathP **bitset.BitSet
		switch head {
		case 'l':
			pathP = &(n.left)
		case 'r':
			pathP = &(n.right)
		default:
			return errors.New("Malformed TrieNode bytedata")
		}

		*pathP = new(bitset.BitSet)
		_, err = (*pathP).ReadFrom(stream)
		if err != nil {
			return err
		}
	}

	return nil
}

// Trie is a dense Merkle Patricia Trie (i.e., all internal nodes have
// two children).
//
// This implementation allows for a "flat" storage by keying nodes on
// their path rather than their hash, resulting in O(1) accesses and
// O(lg n) insertions.
//
// The state trie [specification] describes a sparse Merkle Trie. Note
// that this dense implementation results in an equivalent commitment.
//
// [specification]: https://docs.starknet.io/documentation/develop/State/starknet-state/
type Trie struct {
	height  uint
	root    *bitset.BitSet
	storage TrieStorage
}

func NewTrie(storage TrieStorage, height uint) *Trie {
	return &Trie{
		storage: storage,
		height:  height,
	}
}

// Converts a key to a path that, when followed on a [Trie], leads to the corresponding [TrieNode]
func (t *Trie) PathFromKey(k *felt.Felt) *bitset.BitSet {
	regularK := k.ToRegular()
	return bitset.FromWithLength(t.height, regularK[:])
}

// Finds the set of common MSB bits in two [StoragePath] objects
func FindCommonPath(longerPath, shorterPath *bitset.BitSet) (*bitset.BitSet, bool) {
	divergentBit := uint(0)

	for divergentBit <= shorterPath.Len() &&
		longerPath.Test(longerPath.Len()-divergentBit) == shorterPath.Test(shorterPath.Len()-divergentBit) {
		divergentBit++
	}

	commonPath := shorterPath.Clone()
	for i := uint(0); i < shorterPath.Len()-divergentBit+1; i++ {
		commonPath.DeleteAt(0)
	}
	return commonPath, divergentBit == shorterPath.Len()+1
}

// GetSpecPath returns the suffix of path that diverges from
// parentPath. For example, for a path 0b1011 and parentPath 0b10,
// this function would return the StoragePath object for 0b0.
//
// This is the canonical representation for paths used in the
// [specification]. Since this trie implementation stores nodes by
// path, we need this function to convert paths to their canonical
// representation.
//
// [specification]: https://docs.starknet.io/documentation/develop/State/starknet-state/
func GetSpecPath(path, parentPath *bitset.BitSet) *bitset.BitSet {
	specPath := path.Clone()
	// drop parent path, and one more MSB since left/right relation already encodes that information
	if parentPath != nil {
		specPath.Shrink(specPath.Len() - parentPath.Len() - 1)
		specPath.DeleteAt(specPath.Len() - 1)
	}
	return specPath
}

// step is the on-disk representation of a [TrieNode], where path is the
// key and node is the value.
type step struct {
	path *bitset.BitSet
	node *TrieNode
}

// stepsToRoot enumerates the set of [TrieNode] objects that are on a
// given [StoragePath].
//
// The [step]s are returned in descending order beginning with the root.
func (t *Trie) stepsToRoot(path *bitset.BitSet) ([]step, error) {
	cur := t.root
	steps := []step{}
	for cur != nil {
		node, err := t.storage.Get(cur)
		if err != nil {
			return nil, err
		}

		steps = append(steps, step{
			path: cur,
			node: node,
		})

		_, subset := FindCommonPath(path, cur)
		if cur.Len() >= path.Len() || !subset {
			return steps, nil
		}

		if path.Test(path.Len() - cur.Len() - 1) {
			cur = node.right
		} else {
			cur = node.left
		}
	}

	return steps, nil
}

// Get the corresponding `value` for a `key`
func (t *Trie) Get(key *felt.Felt) (*felt.Felt, error) {
	value, err := t.storage.Get(t.PathFromKey(key))
	if err != nil {
		return nil, err
	}
	return value.value, nil
}

// Update the corresponding `value` for a `key`
func (t *Trie) Put(key *felt.Felt, value *felt.Felt) error {
	path := t.PathFromKey(key)
	node := &TrieNode{
		value: value,
	}

	// empty trie, make new value root
	if t.root == nil {
		if err := t.propagateValues([]step{
			{path: path, node: node},
		}); err != nil {
			return err
		}
		t.root = path
		return nil
	}

	stepsToRoot, err := t.stepsToRoot(path)
	if err != nil {
		return err
	}
	sibling := &stepsToRoot[len(stepsToRoot)-1]

	if path.Equal(sibling.path) {
		sibling.node = node
		if err := t.propagateValues(stepsToRoot); err != nil {
			return err
		}
		return nil
	}

	commonPath, _ := FindCommonPath(path, sibling.path)
	newParent := &TrieNode{
		value: new(felt.Felt),
	}
	if path.Test(path.Len() - commonPath.Len() - 1) {
		newParent.left, newParent.right = sibling.path, path
	} else {
		newParent.left, newParent.right = path, sibling.path
	}

	makeRoot := len(stepsToRoot) == 1
	if !makeRoot { // sibling has a parent
		siblingParent := &stepsToRoot[len(stepsToRoot)-2]

		// replace the link to our sibling with the new parent
		if siblingParent.node.left.Equal(sibling.path) {
			siblingParent.node.left = commonPath
		} else {
			siblingParent.node.right = commonPath
		}
	}

	// replace sibling with new parent
	stepsToRoot[len(stepsToRoot)-1] = step{
		path: commonPath, node: newParent,
	}
	// add new node to steps
	stepsToRoot = append(stepsToRoot, step{
		path: path, node: node,
	})

	// push commitment changes
	if err := t.propagateValues(stepsToRoot); err != nil {
		return err
	} else if makeRoot {
		t.root = commonPath
	}
	return nil
}

// Recalculates [Trie] commitment by propagating `bottom` values as described in the [docs]
//
// [docs]: https://docs.starknet.io/documentation/develop/State/starknet-state/
func (t *Trie) propagateValues(affectedPath []step) error {
	for idx := len(affectedPath) - 1; idx >= 0; idx-- {
		cur := affectedPath[idx]

		if (cur.node.left == nil) != (cur.node.right == nil) {
			panic("should not happen")
		}

		if cur.node.left != nil || cur.node.right != nil {
			// todo: one of the children is already in affectedPath, use that instead of fetching from storage
			left, err := t.storage.Get(cur.node.left)
			if err != nil {
				return err
			}

			right, err := t.storage.Get(cur.node.right)
			if err != nil {
				return err
			}

			leftSpecPath := GetSpecPath(cur.node.left, cur.path)
			rightSpecPath := GetSpecPath(cur.node.right, cur.path)

			cur.node.value, err = crypto.Pedersen(left.Hash(leftSpecPath), right.Hash(rightSpecPath))
			if err != nil {
				return err
			}
		}

		if err := t.storage.Put(cur.path, cur.node); err != nil {
			return err
		}
	}

	return nil
}

// Get commitment of a [Trie]
func (t *Trie) Root() (*felt.Felt, error) {
	root, err := t.storage.Get(t.root)
	if err != nil {
		return nil, err
	}

	specPath := GetSpecPath(t.root, nil)
	return root.Hash(specPath), nil
}

// Try to print a [Trie] in a somewhat human-readable form
func (t *Trie) dump(level int, parentP *bitset.BitSet) {
	if t.root == nil {
		fmt.Printf("%sEMPTY\n", strings.Repeat("\t", level))
		return
	}

	root, err := t.storage.Get(t.root)
	specPath := GetSpecPath(t.root, parentP)
	fmt.Printf("%sstorage : \"%s\" %d spec: \"%s\" %d bottom: \"%s\" \n", strings.Repeat("\t", level), t.root.String(), t.root.Len(), specPath.String(), specPath.Len(), root.value.Text(16))
	if err != nil {
		return
	}
	(&Trie{
		root:    root.left,
		storage: t.storage,
	}).dump(level+1, t.root)
	(&Trie{
		root:    root.right,
		storage: t.storage,
	}).dump(level+1, t.root)
}
