package trie2

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type NodeReader interface {
	Node(owner *felt.Felt, path *Path, hash *felt.Felt) ([]byte, error)
}

type nodeReader struct {
	txn      db.Transaction
	trieType TrieType
}

func (n *nodeReader) Node(owner *felt.Felt, path *Path, hash *felt.Felt) ([]byte, error) {
	panic("implement me")
}

type trieReader struct {
	owner  *felt.Felt
	reader NodeReader
}
