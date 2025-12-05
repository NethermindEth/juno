package trieutils

import (
	"encoding/binary"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
)

func GetNodeByPath(
	r db.KeyValueReader,
	bucket db.Bucket,
	owner *felt.Address,
	path *Path,
	isLeaf bool,
) ([]byte, error) {
	var res []byte
	if err := r.Get(nodeKeyByPath(bucket, owner, path, isLeaf),
		func(value []byte) error {
			res = slices.Clone(value)
			return nil
		},
	); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteNodeByPath(
	w db.KeyValueWriter,
	bucket db.Bucket,
	owner *felt.Address,
	path *Path,
	isLeaf bool,
	blob []byte,
) error {
	return w.Put(nodeKeyByPath(bucket, owner, path, isLeaf), blob)
}

func DeleteNodeByPath(
	w db.KeyValueWriter,
	bucket db.Bucket,
	owner *felt.Address,
	path *Path,
	isLeaf bool,
) error {
	return w.Delete(nodeKeyByPath(bucket, owner, path, isLeaf))
}

func DeleteStorageNodesByPath(w db.KeyValueRangeDeleter, owner *felt.Address) error {
	prefix := db.ContractTrieStorage.Key(owner.Marshal())
	return w.DeleteRange(prefix, dbutils.UpperBound(prefix))
}

func WriteStateID(w db.KeyValueWriter, root *felt.Hash, id uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], id)
	return w.Put(db.StateIDKey((*felt.Felt)(root)), buf[:])
}

func ReadStateID(r db.KeyValueReader, root *felt.Hash) (uint64, error) {
	key := db.StateIDKey((*felt.Felt)(root))

	var id uint64
	if err := r.Get(key, func(value []byte) error {
		id = binary.BigEndian.Uint64(value)
		return nil
	}); err != nil {
		return 0, err
	}

	return id, nil
}

func DeleteStateID(w db.KeyValueWriter, root *felt.Felt) error {
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

func ReadTrieJournal(r db.KeyValueReader) ([]byte, error) {
	var journal []byte
	if err := r.Get(db.TrieJournal.Key(), func(value []byte) error {
		journal = slices.Clone(value)
		return nil
	}); err != nil {
		return nil, err
	}
	return journal, nil
}

func WriteTrieJournal(w db.KeyValueWriter, journal []byte) error {
	return w.Put(db.TrieJournal.Key(), journal)
}

// Construct key bytes to insert a trie node. The format is as follows:
//
// ClassTrie/ContractTrie:
// [1 byte prefix][1 byte node-type][path]
//
// StorageTrie of a Contract :
// [1 byte prefix][32 bytes owner][1 byte node-type][path]
func nodeKeyByPath(prefix db.Bucket, owner *felt.Address, path *Path, isLeaf bool) []byte {
	var (
		prefixBytes = prefix.Key()
		ownerBytes  []byte
		nodeType    []byte
		pathBytes   = path.EncodedBytes()
	)

	if !felt.IsZero(owner) {
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

func GetNodeByHash(
	r db.KeyValueReader,
	bucket db.Bucket,
	owner *felt.Address,
	path *Path,
	hash *felt.Hash,
	isLeaf bool,
) ([]byte, error) {
	var res []byte
	if err := r.Get(nodeKeyByHash(bucket, owner, path, hash, isLeaf),
		func(value []byte) error {
			res = slices.Clone(value)
			return nil
		},
	); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteNodeByHash(
	w db.KeyValueWriter,
	bucket db.Bucket,
	owner *felt.Address,
	path *Path,
	hash *felt.Hash,
	isLeaf bool,
	blob []byte,
) error {
	return w.Put(nodeKeyByHash(bucket, owner, path, hash, isLeaf), blob)
}

// References: https://github.com/NethermindEth/nethermind/pull/6331
// Construct key bytes to insert a trie node. The format is as follows:
//
// ClassTrie :
// [1 byte prefix][1 byte node-type][8 byte from path][32 byte hash]
//
// ContractTrie :
// [1 byte prefix][1 byte node-type][8 byte from path][32 byte hash]
//
// StorageTrie of a Contract :
// [1 byte prefix][32 bytes owner][1 byte node-type][8 byte from path][32 byte hash]
//
// Hash: [Pedersen(path, value) + length] if length > 0 else [value].

func nodeKeyByHash(
	prefix db.Bucket,
	owner *felt.Address,
	path *Path,
	hash *felt.Hash,
	isLeaf bool,
) []byte {
	const pathSignificantBytes = 8
	var (
		prefixBytes = prefix.Key()
		ownerBytes  []byte
		nodeType    []byte
		pathBytes   = path.ActiveBytes()
		hashBytes   = hash.Bytes()
	)

	if !felt.IsZero(owner) {
		ob := owner.Bytes()
		ownerBytes = ob[:]
	}

	if isLeaf {
		nodeType = leaf.Bytes()
	} else {
		nodeType = nonLeaf.Bytes()
	}

	keySize := len(prefixBytes) + len(ownerBytes) + len(nodeType) + len(hashBytes)
	if len(pathBytes) < pathSignificantBytes {
		keySize += pathSignificantBytes
	} else {
		keySize += len(pathBytes)
	}

	key := make([]byte, 0, keySize)
	key = append(key, prefixBytes...)
	key = append(key, ownerBytes...)
	key = append(key, nodeType...)

	if len(pathBytes) > 0 {
		if len(pathBytes) < pathSignificantBytes {
			key = append(key, pathBytes...)
			key = append(key, make([]byte, pathSignificantBytes-len(pathBytes))...)
		} else {
			key = append(key, pathBytes...)
		}
	} else {
		key = append(key, make([]byte, pathSignificantBytes)...)
	}

	key = append(key, hashBytes[:]...)

	return key
}
