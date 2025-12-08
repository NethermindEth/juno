package trie2

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type nodeReader struct {
	id     trieutils.TrieID
	reader database.NodeReader
}

func newNodeReader(id trieutils.TrieID, nodeDB database.NodeDatabase) (nodeReader, error) {
	reader, err := nodeDB.NodeReader(id)
	if err != nil {
		return nodeReader{}, err
	}
	return nodeReader{id: id, reader: reader}, nil
}

func (r *nodeReader) node(path trieutils.Path, hash *felt.Hash, isLeaf bool) ([]byte, error) {
	if r.reader == nil {
		return nil, &MissingNodeError{tt: r.id.Type(), owner: r.id.Owner(), path: path, hash: *hash}
	}
	owner := r.id.Owner()
	return r.reader.Node(&owner, &path, hash, isLeaf)
}

func NewEmptyNodeReader() nodeReader {
	return nodeReader{id: trieutils.NewEmptyTrieID(felt.StateRootHash{}), reader: nil}
}
