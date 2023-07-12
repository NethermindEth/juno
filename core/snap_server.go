package core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

type TrieRootInfo struct {
	StorageRoot *felt.Felt
	ClassRoot   *felt.Felt
}

type ClassRangeResult struct {
	Paths       []*felt.Felt
	ClassHashes []*felt.Felt
	Classes     []*Class

	Proofs []trie.ProofNode
}

type AddressRangeResult struct {
	Paths  []*felt.Felt
	Hashes []*felt.Felt
	Leaves []*AddressRangeLeaf

	Proofs []trie.ProofNode
}

type AddressRangeLeaf struct {
	StorageRoot *felt.Felt
	ClassHash   *felt.Felt
	Nonce       *felt.Felt
}

type StorageRangeRequest struct {
	Path      *felt.Felt
	Hash      *felt.Felt
	StartAddr *felt.Felt
	LimitAddr *felt.Felt
}

type StorageRangeResult struct {
	Paths  []*felt.Felt
	Values []*felt.Felt

	Proofs []trie.ProofNode
}

type SnapServer interface {
	GetTrieRootAt(blockHash *felt.Felt) (TrieRootInfo, error)
	GetClassRange(classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (ClassRangeResult, error)
	GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt) (AddressRangeResult, error)
	GetContractRange(storageTrieRootHash *felt.Felt, requests []*StorageRangeRequest) ([]*StorageRangeResult, error)
}
