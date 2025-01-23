package trie2

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type TrieType uint8

const (
	Empty TrieType = iota
	ClassTrie
	ContractTrie
)

// Represents the identifier for uniquely identifying a trie.
type ID struct {
	TrieType    TrieType
	Root        felt.Felt // The root hash of the trie
	Owner       felt.Felt // The contract address which the trie belongs to
	StorageRoot felt.Felt // The root hash of the storage trie of a contract.
}

// Returns the corresponding DB bucket for the trie
func (id *ID) Bucket() db.Bucket {
	switch id.TrieType {
	case ClassTrie:
		return db.ClassTrie
	case ContractTrie:
		if id.Owner == (felt.Felt{}) {
			return db.ContractTrieContract
		}
		return db.ContractTrieStorage
	case Empty:
		return db.Bucket(0)
	default:
		panic("invalid trie type")
	}
}

// Constructs an identifier for a class trie with the provided class trie root hash
func ClassTrieID(root felt.Felt) *ID {
	return &ID{
		TrieType:    ClassTrie,
		Root:        root,
		Owner:       felt.Zero, // class trie does not have an owner
		StorageRoot: felt.Zero, // only contract storage trie has a storage root
	}
}

// Constructs an identifier for a contract trie or a contract's storage trie
func ContractTrieID(root, owner, storageRoot felt.Felt) *ID {
	return &ID{
		TrieType:    ContractTrie,
		Root:        root,
		Owner:       owner,
		StorageRoot: storageRoot,
	}
}

// A general identifier, typically used for temporary trie
func TrieID(root felt.Felt) *ID {
	return &ID{
		TrieType:    Empty,
		Root:        root,
		Owner:       felt.Zero,
		StorageRoot: felt.Zero,
	}
}
