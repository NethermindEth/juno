package rpcv10

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"go.uber.org/zap"
)

type StorageResponseFlags struct {
	IncludeLastUpdateBlock bool
}

func (f *StorageResponseFlags) UnmarshalJSON(data []byte) error {
	var flags []string
	if err := json.Unmarshal(data, &flags); err != nil {
		return err
	}
	*f = StorageResponseFlags{}

	for _, flag := range flags {
		switch flag {
		case "INCLUDE_LAST_UPDATE_BLOCK":
			f.IncludeLastUpdateBlock = true
		default:
			return fmt.Errorf("unknown storage response flag: %s", flag)
		}
	}

	return nil
}

// @todo add marshal and unmarshal logic to make it follow the spec
type StorageResult struct {
	Value           felt.Felt `json:"value"`
	LastUpdateBlock uint64    `json:"last_update_block"`
}

// @todo verify the entire logic, partially created by Claude

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/d6dc6ad31a1bb61c287d862ca4bdef4eb66a59a2/api/starknet_api_openrpc.json#L197
func (h *Handler) StorageAt(
	address, key *felt.Felt, id *BlockID, flags StorageResponseFlags,
) (StorageResult, *jsonrpc.Error) {
	var result StorageResult
	stateReader, stateCloser, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return result, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageAt")

	_, err := stateReader.ContractClassHash(address)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return result, rpccore.ErrContractNotFound
		}
		h.log.Error("Failed to get contract class hash", zap.Error(err))
		return result, rpccore.ErrInternal.CloneWithData(err)
	}

	value, err := stateReader.ContractStorage(address, key)
	if err != nil {
		return result, rpccore.ErrInternal.CloneWithData(err)
	}

	result.Value = value

	if !flags.IncludeLastUpdateBlock {
		return result, nil
	}

	blockNumber, rpcErr := h.resolveBlockNumber(id)
	if rpcErr != nil {
		return result, rpcErr
	}

	lastUpdateBlock, rpcErr := h.findStorageLastUpdateBlock(address, key, blockNumber)
	if rpcErr != nil {
		return result, rpcErr
	}

	result.LastUpdateBlock = lastUpdateBlock
	return result, nil
}

func (h *Handler) resolveBlockNumber(id *BlockID) (uint64, *jsonrpc.Error) {
	switch {
	case id.IsPreConfirmed():
		pending, err := h.PendingData()
		if err != nil {
			return 0, rpccore.ErrBlockNotFound
		}
		return pending.GetBlock().Number, nil
	case id.IsLatest():
		header, err := h.bcReader.HeadsHeader()
		if err != nil {
			return 0, rpccore.ErrBlockNotFound
		}
		return header.Number, nil
	case id.IsHash():
		num, err := h.bcReader.BlockNumberByHash(id.Hash())
		if err != nil {
			return 0, rpccore.ErrBlockNotFound
		}
		return num, nil
	case id.IsNumber():
		return id.Number(), nil
	case id.IsL1Accepted():
		l1Head, err := h.bcReader.L1Head()
		if err != nil {
			return 0, rpccore.ErrBlockNotFound
		}
		return l1Head.BlockNumber, nil
	default:
		panic("unknown block id type")
	}
}

// findStorageLastUpdateBlock iterates backwards through state updates to find
// the most recent block that modified the given storage slot.
func (h *Handler) findStorageLastUpdateBlock(
	address, key *felt.Felt, upToBlock uint64,
) (uint64, *jsonrpc.Error) {
	for blockNum := int64(upToBlock); blockNum >= 0; blockNum-- {
		stateUpdate, err := h.bcReader.StateUpdateByNumber(uint64(blockNum))
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				continue
			}
			h.log.Error("Failed to get state update", zap.Error(err))
			return 0, rpccore.ErrInternal
		}

		if storageDiff, ok := stateUpdate.StateDiff.StorageDiffs[*address]; ok {
			if _, ok := storageDiff[*key]; ok {
				return uint64(blockNum), nil
			}
		}
	}

	return 0, nil
}
