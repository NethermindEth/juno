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
		panic("unknown trie type")
	}
}

// A unique identifier for a trie type
type TrieID interface {
	// The state commitment where this trie belongs to. Note that this is not the trie root hash.
	// Also, note that a state commitment is calculated with the combination of both class trie and contract trie.
	StateComm() felt.Hash

	HasOwner() bool      // whether the trie has an owner
	Owner() felt.Address // the owner of the trie (e.g. contract address)

	Type() TrieType
	Bucket() db.Bucket // the database bucket prefix
}

// Identifier for a class trie
type ClassTrieID struct {
	stateComm felt.Hash
}

func NewClassTrieID(stateComm felt.Hash) ClassTrieID {
	return ClassTrieID{stateComm: stateComm}
}

func (id ClassTrieID) Type() TrieType       { return Class }
func (id ClassTrieID) Bucket() db.Bucket    { return db.ClassTrie }
func (id ClassTrieID) StateComm() felt.Hash { return id.stateComm }
func (id ClassTrieID) HasOwner() bool       { return false }
func (id ClassTrieID) Owner() felt.Address  { return felt.Address{} }

// Identifier for a contract trie
type ContractTrieID struct {
	stateComm felt.Hash
}

func NewContractTrieID(stateComm felt.Hash) ContractTrieID {
	return ContractTrieID{stateComm: stateComm}
}

func (id ContractTrieID) Type() TrieType       { return Contract }
func (id ContractTrieID) Bucket() db.Bucket    { return db.ContractTrieContract }
func (id ContractTrieID) StateComm() felt.Hash { return id.stateComm }
func (id ContractTrieID) HasOwner() bool       { return false }
func (id ContractTrieID) Owner() felt.Address  { return felt.Address{} }

// Identifier for a contract storage trie
type ContractStorageTrieID struct {
	stateComm felt.Hash
	owner     felt.Address
}

func NewContractStorageTrieID(stateComm felt.Hash, owner felt.Address) ContractStorageTrieID {
	return ContractStorageTrieID{stateComm: stateComm, owner: owner}
}

func (id ContractStorageTrieID) Type() TrieType       { return ContractStorage }
func (id ContractStorageTrieID) Bucket() db.Bucket    { return db.ContractTrieStorage }
func (id ContractStorageTrieID) StateComm() felt.Hash { return id.stateComm }
func (id ContractStorageTrieID) HasOwner() bool       { return true }
func (id ContractStorageTrieID) Owner() felt.Address  { return id.owner }

// Identifier for an empty trie, only used for temporary purposes
type EmptyTrieID struct {
	stateComm felt.Hash
}

func NewEmptyTrieID(stateComm felt.Hash) EmptyTrieID {
	return EmptyTrieID{stateComm: stateComm}
}

func (id EmptyTrieID) Type() TrieType       { return Empty }
func (id EmptyTrieID) Bucket() db.Bucket    { return db.Bucket(0) }
func (id EmptyTrieID) StateComm() felt.Hash { return id.stateComm }
func (id EmptyTrieID) HasOwner() bool       { return false }
func (id EmptyTrieID) Owner() felt.Address  { return felt.Address{} }
