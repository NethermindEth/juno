package history

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

func parseBlockKey(key, prefix []byte) (uint64, error) {
	if len(key) != len(prefix)+8 {
		return 0, fmt.Errorf("malformed block-keyed entry: key len %d, want %d", len(key), len(prefix)+8)
	}
	return binary.BigEndian.Uint64(key[len(prefix):]), nil
}

func parseStorageKey(key, prefix []byte) (felt.Felt, uint64, error) {
	if len(key) != len(prefix)+felt.Bytes+8 {
		return felt.Felt{}, 0, fmt.Errorf(
			"malformed storage-history entry: key len %d, want %d",
			len(key),
			len(prefix)+felt.Bytes+8,
		)
	}
	slot := felt.FromBytes[felt.Felt](key[len(prefix) : len(prefix)+felt.Bytes])
	block := binary.BigEndian.Uint64(key[len(prefix)+felt.Bytes:])
	return slot, block, nil
}
