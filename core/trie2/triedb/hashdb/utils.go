package hashdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

// key = hash (32 bytes) + path (dynamic) + pathLen (1 byte)
func nodeKey(path *trieutils.Path, hash *felt.Hash) []byte {
	hashBytes := hash.Bytes()
	pathBytes := path.EncodedBytes()

	key := make([]byte, felt.Bytes+len(pathBytes))

	copy(key[:felt.Bytes], hashBytes[:])

	copy(key[felt.Bytes:], pathBytes)
	return key
}

func decodeNodeKey(key []byte) (path trieutils.Path, hash felt.Hash, err error) {
	hash.Unmarshal(key[:felt.Bytes])

	pathBytes := key[felt.Bytes:]
	if err := path.UnmarshalBinary(pathBytes); err != nil {
		return path, hash, err
	}

	return path, hash, nil
}
