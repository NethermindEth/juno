package starknet

import (
	"errors"
	"regexp"

	"github.com/NethermindEth/juno/internal/db/block"

	"github.com/NethermindEth/juno/pkg/types"
)

var (
	feltRegexp = regexp.MustCompile(`^0x0[a-fA-F0-9]{1,63}$`)
	blockTags  = map[string]any{
		"latest":  nil,
		"pending": nil,
	}
	storageKeyRegexp = regexp.MustCompile("^0x0[0-7]{1}[a-fA-F0-9]{0,62}$")
)

func isFelt(s string) bool {
	return feltRegexp.MatchString(s)
}

func isBlockTag(s string) bool {
	_, ok := blockTags[s]
	return ok
}

func isStorageKey(s string) bool {
	return storageKeyRegexp.MatchString(s)
}

func getBlockById(blockId *BlockId, blockManager *block.Manager) (*types.Block, error) {
    if blockId == nil {
        return nil, NewInvalidBlockId()
    }
	switch blockId.idType {
	case blockIdHash:
		hash, _ := blockId.hash()
		return blockManager.GetBlockByHash(hash)
	case blockIdNumber:
		number, _ := blockId.number()
		return blockManager.GetBlockByNumber(number)
	case blockIdTag:
		return nil, errors.New("not implemented")
	default:
		return nil, NewInvalidBlockId()
	}
}
