package trieutils

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
)

func GetNodeByPath(r db.KeyValueReader, bucket db.Bucket, owner *felt.Felt, path *Path, isLeaf bool) ([]byte, error) {
	var res []byte
	if err := r.Get(nodeKeyByPath(bucket, owner, path, isLeaf),
		func(value []byte) error {
			res = value
			return nil
		},
	); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteNodeByPath(w db.KeyValueWriter, bucket db.Bucket, owner *felt.Felt, path *Path, isLeaf bool, blob []byte) error {
	return w.Put(nodeKeyByPath(bucket, owner, path, isLeaf), blob)
}

func DeleteNodeByPath(w db.KeyValueWriter, bucket db.Bucket, owner *felt.Felt, path *Path, isLeaf bool) error {
	return w.Delete(nodeKeyByPath(bucket, owner, path, isLeaf))
}

func DeleteStorageNodesByPath(w db.KeyValueRangeDeleter, owner felt.Felt) error {
	prefix := db.ContractTrieStorage.Key(owner.Marshal())
	return w.DeleteRange(prefix, dbutils.UpperBound(prefix))
}

// Construct key bytes to insert a trie node. The format is as follows:
//
// ClassTrie/ContractTrie:
// [1 byte prefix][1 byte node-type][path]
//
// StorageTrie of a Contract :
// [1 byte prefix][32 bytes owner][1 byte node-type][path]
func nodeKeyByPath(prefix db.Bucket, owner *felt.Felt, path *Path, isLeaf bool) []byte {
	var (
		prefixBytes = prefix.Key()
		ownerBytes  []byte
		nodeType    []byte
		pathBytes   = path.EncodedBytes()
	)

	if !owner.IsZero() {
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

func GetNodeByHash(r db.KeyValueReader, bucket db.Bucket, owner felt.Felt, path Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	var res []byte
	if err := r.Get(NodeKeyByHash(bucket, owner, path, hash, isLeaf),
		func(value []byte) error {
			res = value
			return nil
		},
	); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteNodeByHash(w db.KeyValueWriter, bucket db.Bucket, owner felt.Felt, path Path, hash felt.Felt, isLeaf bool, blob []byte) error {
	return w.Put(NodeKeyByHash(bucket, owner, path, hash, isLeaf), blob)
}

func DeleteNodeByHash(w db.KeyValueWriter, bucket db.Bucket, owner felt.Felt, path Path, hash felt.Felt, isLeaf bool) error {
	return w.Delete(NodeKeyByHash(bucket, owner, path, hash, isLeaf))
}

// References: https://github.com/NethermindEth/nethermind/pull/6331
// Construct key bytes to insert a trie node. The format is as follows:
//
// ClassTrie :
// [1 byte prefix][1 section byte][1 byte node-type][8 byte from path][1 path length byte][32 byte hash]
//
// ContractTrie :
// [1 byte prefix][1 section byte][1 byte node-type][8 byte from path][1 path length byte][32 byte hash]
//
// StorageTrie of a Contract :
// [1 byte prefix][1 section byte][32 bytes owner][1 byte node-type][8 byte from path][1 path length byte][32 byte hash]
//
// Section:
// 0 if state and path length is <= 5.
// 1 if state and path length is > 5.
// 2 if storage.
//
// Hash: [Pedersen(path, value) + length] if length > 0 else [value].

func NodeKeyByHash(prefix db.Bucket, owner felt.Felt, path Path, hash felt.Felt, isLeaf bool) []byte {
	const (
		pathSignificantBytes = 8
		shortPathLength      = 5
	)
	var (
		prefixBytes = prefix.Key()
		ownerBytes  []byte
		nodeType    []byte
		pathBytes   = path.EncodedBytes()
		hashBytes   = hash.Bytes()
		section     byte
	)

	if prefix == db.ContractTrieStorage {
		section = 2
	} else {
		if path.Len() <= shortPathLength {
			section = 0
		} else {
			section = 1
		}
	}

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
	key = append(key, section)
	key = append(key, ownerBytes...)
	key = append(key, nodeType...)
	key = append(key, pathBytes[0:pathSignificantBytes]...)
	key = append(key, byte(len(pathBytes)))
	key = append(key, hashBytes[:]...)

	return key
}
