package trie2

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

var (
	_ TrieID = (*ClassTrieID)(nil)
	_ TrieID = (*ContractTrieID)(nil)
	_ TrieID = (*ContractStorageTrieID)(nil)
	_ TrieID = (*EmptyTrieID)(nil)
)

type TrieType uint8

const (
	Empty TrieType = iota
	ClassTrie
	ContractTrie
)

// Represents the identifier for uniquely identifying a trie.
type ID struct {
	TrieType TrieType
	Owner    felt.Felt // The contract address which the trie belongs to
}

// Represents a class trie
type ClassTrieID struct{}

func NewClassTrieID() *ClassTrieID          { return &ClassTrieID{} }
func (ClassTrieID) Bucket() db.Bucket       { return db.ClassTrie }
func (ClassTrieID) Owner() felt.Felt        { return felt.Zero }
func (ClassTrieID) IsContractStorage() bool { return false }

// Represents a contract trie
type ContractTrieID struct{}

func NewContractTrieID() *ContractTrieID          { return &ContractTrieID{} }
func (id ContractTrieID) Bucket() db.Bucket       { return db.ContractTrieContract }
func (id ContractTrieID) Owner() felt.Felt        { return felt.Zero }
func (id ContractTrieID) IsContractStorage() bool { return false }

// Represents a contract storage trie
type ContractStorageTrieID struct {
	owner felt.Felt
}

func NewContractStorageTrieID(owner felt.Felt) *ContractStorageTrieID {
	return &ContractStorageTrieID{owner: owner}
}

func NewContractStorageTrieIDFromFelt(owner felt.Felt) *ContractStorageTrieID {
	return &ContractStorageTrieID{owner: owner}
}

func (id ContractStorageTrieID) Bucket() db.Bucket       { return db.ContractTrieStorage }
func (id ContractStorageTrieID) Owner() felt.Felt        { return id.owner }
func (id ContractStorageTrieID) IsContractStorage() bool { return true }

// Represents an empty trie, only used for temporary purposes
type EmptyTrieID struct{}

func NewEmptyTrieID() *EmptyTrieID          { return &EmptyTrieID{} }
func (EmptyTrieID) Bucket() db.Bucket       { return db.Bucket(0) }
func (EmptyTrieID) Owner() felt.Felt        { return felt.Zero }
func (EmptyTrieID) IsContractStorage() bool { return false }
