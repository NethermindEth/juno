package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
)

type (
	StoragePath  = bitset.BitSet
	StorageValue = TrieNode
)

type (
	TrieKey   = felt.Felt
	TrieValue = felt.Felt
)

type TrieStorage interface {
	Put(key *StoragePath, value *StorageValue) error
	Get(key *StoragePath) (*StorageValue, error)
}

type TrieNode struct {
	value *TrieValue
	left  *StoragePath
	right *StoragePath
}

func (n *TrieNode) Hash(specPath *StoragePath) *TrieValue {
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
	hash, err := Pedersen(n.value, &pathFelt)
	if err != nil {
		panic("Pedersen failed TrieNode.Hash")
	}

	pathFelt.SetUint64(uint64(specPath.Len()))
	return hash.Add(hash, &pathFelt)
}

func (n *TrieNode) Equal(other *TrieNode) bool {
	return n.value.Equal(other.value) && n.left.Equal(other.left) && n.right.Equal(n.right)
}

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

func (n *TrieNode) UnmarshalBinary(data []byte) error {
	if len(data) < felt.Bytes {
		return errors.New("Malformed TrieNode bytedata")
	}
	n.value = new(TrieValue).SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]

	stream := bytes.NewReader(data)
	for stream.Len() > 0 {
		head, err := stream.ReadByte()
		if err != nil {
			return err
		}

		var pathP **StoragePath
		switch head {
		case 'l':
			pathP = &(n.left)
		case 'r':
			pathP = &(n.right)
		default:
			return errors.New("Malformed TrieNode bytedata")
		}

		*pathP = new(StoragePath)
		_, err = (*pathP).ReadFrom(stream)
		if err != nil {
			return err
		}
	}

	return nil
}

type Trie struct {
	root    *StoragePath
	storage TrieStorage
}

func PathFromKey(k *TrieKey) *StoragePath {
	regularK := k.ToRegular()
	return bitset.FromWithLength(felt.Bits, regularK[:])
}

func FindCommonPath(longerPath, shorterPath *StoragePath) (*StoragePath, bool) {
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

// calculates node paths according to the [docs]
//
// [docs]: https://docs.starknet.io/documentation/develop/State/starknet-state/
func GetSpecPath(path, parentPath *StoragePath) *StoragePath {
	specPath := path.Clone()
	// drop parent path, and one more MSB since left/right relation already encodes that information
	if parentPath != nil {
		specPath.Shrink(specPath.Len() - parentPath.Len() - 1)
		specPath.DeleteAt(specPath.Len() - 1)
	}
	return specPath
}

type step = struct {
	path *StoragePath
	node *StorageValue
}

func (t *Trie) stepsToRoot(path *StoragePath) ([]step, error) {
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

func (t *Trie) Get(key *TrieKey) (*TrieValue, error) {
	value, err := t.storage.Get(PathFromKey(key))
	if err != nil {
		return nil, err
	}
	return value.value, nil
}

func (t *Trie) Put(key *TrieKey, value *TrieValue) error {
	path := PathFromKey(key)
	node := &TrieNode{
		value: value,
	}

	if err := t.storage.Put(path, node); err != nil {
		return err
	}

	// empty trie, make new value root
	if t.root == nil {
		t.root = path
		return nil
	}

	stepsToRoot, err := t.stepsToRoot(path)
	if err != nil {
		return err
	}
	sibling := stepsToRoot[len(stepsToRoot)-1]

	if path.Equal(sibling.path) {
		return nil
	}

	commonPath, _ := FindCommonPath(path, sibling.path)
	newParent := &TrieNode{
		value: new(TrieValue),
	}
	if path.Test(path.Len() - commonPath.Len() - 1) {
		newParent.left, newParent.right = sibling.path, path
	} else {
		newParent.left, newParent.right = path, sibling.path
	}

	if err := t.storage.Put(commonPath, newParent); err != nil {
		return err
	}

	if len(stepsToRoot) > 1 { // sibling has a parent
		siblingParent := stepsToRoot[len(stepsToRoot)-2]

		// replace the link to our sibling with the new parent
		if siblingParent.node.left.Equal(sibling.path) {
			siblingParent.node.left = commonPath
		} else {
			siblingParent.node.right = commonPath
		}
		if err := t.storage.Put(siblingParent.path, siblingParent.node); err != nil {
			return err
		}
	} else { // sibling was the root, make new parent the root
		t.root = commonPath
	}

	return nil
}

func (t *Trie) dump(level int, parentP *StoragePath) {
	if t.root == nil {
		fmt.Printf("%sEMPTY\n", strings.Repeat("\t", level))
		return
	}

	root, err := t.storage.Get(t.root)
	specPath := GetSpecPath(t.root, parentP)
	fmt.Printf("%s\"%s\" %d bottom: \"%s\" \n", strings.Repeat("\t", level), specPath.DumpAsBits(), specPath.Len(), root.value.Text(16))
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
