package trieutils

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

// Represents the type of trie on Starknet
type TrieType int

const (
	Empty TrieType = iota
	Class
	Contract
	ContractStorage
)

func (t TrieType) String() string {
	switch t {
	case Empty:
		return "Empty"
	case Class:
		return "Class"
	case Contract:
		return "Contract"
	case ContractStorage:
		return "ContractStorage"
	default:
		return "Unknown"
	}
}

// A unique identifier for a trie type
type TrieID interface {
	Type() TrieType
	Bucket() db.Bucket
	HasOwner() bool
	Owner() felt.Felt
}

// Identifier for a class trie
type ClassTrieID struct {
	stateRoot felt.Felt
}

func NewClassTrieID(stateRoot felt.Felt) *ClassTrieID {
	return &ClassTrieID{stateRoot: stateRoot}
}

func (id ClassTrieID) Type() TrieType    { return Class }
func (id ClassTrieID) Bucket() db.Bucket { return db.ClassTrie }
func (id ClassTrieID) HasOwner() bool    { return false }
func (id ClassTrieID) Owner() felt.Felt  { return id.stateRoot }

// Identifier for a contract trie
type ContractTrieID struct {
	stateRoot felt.Felt
}

func NewContractTrieID(stateRoot felt.Felt) *ContractTrieID {
	return &ContractTrieID{stateRoot: stateRoot}
}

func (id ContractTrieID) Type() TrieType    { return Contract }
func (id ContractTrieID) Bucket() db.Bucket { return db.ContractTrieContract }
func (id ContractTrieID) HasOwner() bool    { return false }
func (id ContractTrieID) Owner() felt.Felt  { return id.stateRoot }

// Identifier for a contract storage trie
type ContractStorageTrieID struct {
	stateRoot felt.Felt
	owner     felt.Felt
}

func NewContractStorageTrieID(stateRoot, owner felt.Felt) *ContractStorageTrieID {
	return &ContractStorageTrieID{stateRoot: stateRoot, owner: owner}
}

func (id ContractStorageTrieID) Type() TrieType    { return ContractStorage }
func (id ContractStorageTrieID) Bucket() db.Bucket { return db.ContractTrieStorage }
func (id ContractStorageTrieID) HasOwner() bool    { return true }
func (id ContractStorageTrieID) Owner() felt.Felt  { return id.owner }

// Identifier for an empty trie, only used for temporary purposes
type EmptyTrieID struct{}

func NewEmptyTrieID() *EmptyTrieID    { return &EmptyTrieID{} }
func (EmptyTrieID) Type() TrieType    { return Empty }
func (EmptyTrieID) Bucket() db.Bucket { return db.Bucket(0) }
func (EmptyTrieID) HasOwner() bool    { return false }
func (EmptyTrieID) Owner() felt.Felt  { return felt.Zero }
