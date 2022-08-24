package starknet

import (
	"errors"
	"regexp"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/block"
	"go.uber.org/zap"

	"github.com/NethermindEth/juno/pkg/jsonrpc"
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

func getBlockById(blockId *BlockId, blockManager *block.Manager, logger *zap.SugaredLogger) (block *types.Block, err error) {
	if blockId == nil {
		return nil, InvalidBlockId
	}
	switch blockId.idType {
	case blockIdHash:
		hash, _ := blockId.hash()
		block, err = blockManager.GetBlockByHash(hash)
	case blockIdNumber:
		number, _ := blockId.number()
		block, err = blockManager.GetBlockByNumber(number)
	case blockIdTag:
		return nil, errors.New("not implemented")
	default:
		return nil, InvalidBlockId
	}
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, InvalidBlockId
		}
		logger.With("err", err).Errorf("failed to get block with id: %v", blockId)
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	return block, nil
}
