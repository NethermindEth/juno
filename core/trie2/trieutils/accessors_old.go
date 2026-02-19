package trieutils

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// changed by the new implementation
func (l leafType) Bytes() []byte {
	return []byte{byte(l)}
}

func nodeKeyByPathOld(prefix db.Bucket, owner *felt.Address, path *Path, isLeaf bool) []byte {
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

func nodeKeyByHashOld(
	prefix db.Bucket,
	owner *felt.Address,
	path *BitArrayOld,
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
