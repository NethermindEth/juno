package trieutils

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

func GetNodeByPath(r db.KeyValueReader, id TrieID, path Path, isLeaf bool) ([]byte, error) {
	var res []byte
	if err := r.Get(nodeKeyByPath(id.Bucket(), id.Owner(), path, isLeaf),
		func(value []byte) error {
			res = value
			return nil
		},
	); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteNodeByPath(w db.KeyValueWriter, id TrieID, path Path, isLeaf bool, blob []byte) error {
	return w.Put(nodeKeyByPath(id.Bucket(), id.Owner(), path, isLeaf), blob)
}

func DeleteNodeByPath(w db.KeyValueWriter, id TrieID, path Path, isLeaf bool) error {
	return w.Delete(nodeKeyByPath(id.Bucket(), id.Owner(), path, isLeaf))
}

// Construct key bytes to insert a trie node. The format is as follows:
//
// ClassTrie/ContractTrie:
// [1 byte prefix][1 byte node-type][path]
//
// StorageTrie of a Contract :
// [1 byte prefix][32 bytes owner][1 byte node-type][path]
func nodeKeyByPath(prefix db.Bucket, owner felt.Felt, path Path, isLeaf bool) []byte {
	var (
		prefixBytes = prefix.Key()
		ownerBytes  []byte
		nodeType    []byte
		pathBytes   = path.EncodedBytes()
	)

	if !owner.Equal(&felt.Zero) {
		ob := owner.Bytes()
		ownerBytes = ob[:]
	}

	if isLeaf {
		nodeType = leaf.Bytes()
	} else {
		nodeType = nonLeaf.Bytes()
	}

	key := make([]byte, 0, len(prefixBytes)+len(ownerBytes)+len(nodeType)+len(pathBytes))
	key = append(key, prefixBytes...)
	key = append(key, ownerBytes...)
	key = append(key, nodeType...)
	key = append(key, pathBytes...)

	return key
}
