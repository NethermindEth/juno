package core

import (
	"bytes"
	"errors"

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
	root    StoragePath
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
