package trieutils

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
)

func GetNodeByPath(r db.KeyValueReader, bucket db.Bucket, owner felt.Felt, path Path, isLeaf bool) ([]byte, error) {
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

func WriteNodeByPath(w db.KeyValueWriter, bucket db.Bucket, owner felt.Felt, path Path, isLeaf bool, blob []byte) error {
	return w.Put(nodeKeyByPath(bucket, owner, path, isLeaf), blob)
}

func DeleteNodeByPath(w db.KeyValueWriter, bucket db.Bucket, owner felt.Felt, path Path, isLeaf bool) error {
	return w.Delete(nodeKeyByPath(bucket, owner, path, isLeaf))
}

func DeleteStorageNodesByPath(w db.KeyValueRangeDeleter, owner felt.Felt) error {
	prefix := db.ContractTrieStorage.Key(owner.Marshal())
	return w.DeleteRange(prefix, dbutils.UpperBound(prefix))
}

func WriteStateID(w db.KeyValueWriter, root felt.Felt, id uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], id)
	return w.Put(db.StateIDKey(root), buf[:])
}

func ReadStateID(r db.KeyValueReader, root felt.Felt) (uint64, error) {
	key := db.StateIDKey(root)

	var id uint64
	if err := r.Get(key, func(value []byte) error {
		id = binary.BigEndian.Uint64(value)
		return nil
	}); err != nil {
		return 0, err
	}

	return id, nil
}

func DeleteStateID(w db.KeyValueWriter, root felt.Felt) error {
	return w.Delete(db.StateIDKey(root))
}

func ReadPersistedStateID(r db.KeyValueReader) (uint64, error) {
	var id uint64
	if err := r.Get(db.PersistedStateID.Key(), func(value []byte) error {
		id = binary.BigEndian.Uint64(value)
		return nil
	}); err != nil {
		return 0, err
	}
	return id, nil
}

func WritePersistedStateID(w db.KeyValueWriter, id uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], id)
	return w.Put(db.PersistedStateID.Key(), buf[:])
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
