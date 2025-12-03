package database

import (
	"io"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type TrieDBScheme uint8

const (
	RawScheme TrieDBScheme = iota + 1
	PathScheme
	HashScheme
)

// Represents a reader for trie nodes
type NodeReader interface {
	Node(owner *felt.Felt, path *trieutils.Path, hash *felt.Felt, isLeaf bool) ([]byte, error)
}

// Represents a database that produces a node reader for a given trie id
type NodeDatabase interface {
	NodeReader(id trieutils.TrieID) (NodeReader, error)
}

type NodeIterator interface {
	NewIterator(id trieutils.TrieID) (db.Iterator, error)
}

// Represents a database that access all things related to a trie
type TrieDB interface {
	NodeDatabase
	NodeIterator
	io.Closer

	Commit(stateComm *felt.Felt) error
	Update(
		root,
		parent *felt.Felt,
		blockNum uint64,
		mergeClassNodes,
		mergeContractNodes *trienode.MergeNodeSet,
	) error
	Scheme() TrieDBScheme
}
