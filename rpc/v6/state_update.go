package rpcv6

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/sync"
)

// https://github.com/starkware-libs/starknet-specs/blob/8016dd08ed7cd220168db16f24c8a6827ab88317/api/starknet_api_openrpc.json#L909
type StateUpdate struct {
	BlockHash *felt.Felt       `json:"block_hash,omitempty"`
	NewRoot   *felt.Felt       `json:"new_root,omitempty"`
	OldRoot   *felt.Felt       `json:"old_root"`
	StateDiff *rpcv8.StateDiff `json:"state_diff"`
}

/****************************************************
		StateUpdate Handlers
*****************************************************/

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L77
func (h *Handler) StateUpdate(id BlockID) (*StateUpdate, *jsonrpc.Error) {
	var update *core.StateUpdate
	var err error
	if id.Latest {
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			err = heightErr
		} else {
			update, err = h.bcReader.StateUpdateByNumber(height)
		}
	} else if id.Pending {
		var pending *sync.Pending
		pending, err = h.syncReader.Pending()
		if err == nil {
			update = pending.StateUpdate
		}
	} else if id.Hash != nil {
		update, err = h.bcReader.StateUpdateByHash(id.Hash)
	} else {
		update, err = h.bcReader.StateUpdateByNumber(id.Number)
	}
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, sync.ErrPendingBlockNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	nonces := make([]rpcv8.Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, rpcv8.Nonce{ContractAddress: addr, Nonce: *nonce})
	}

	storageDiffs := make([]rpcv8.StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]rpcv8.Entry, 0, len(diffs))
		for key, value := range diffs {
			entries = append(entries, rpcv8.Entry{
				Key:   key,
				Value: *value,
			})
		}

		storageDiffs = append(storageDiffs, rpcv8.StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		})
	}

	deployedContracts := make([]rpcv8.DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, rpcv8.DeployedContract{
			Address:   addr,
			ClassHash: *classHash,
		})
	}

	declaredClasses := make([]rpcv8.DeclaredClass, 0, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses = append(declaredClasses, rpcv8.DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		})
	}

	replacedClasses := make([]rpcv8.ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		replacedClasses = append(replacedClasses, rpcv8.ReplacedClass{
			ClassHash:       *classHash,
			ContractAddress: addr,
		})
	}

	return &StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   update.OldRoot,
		NewRoot:   update.NewRoot,
		StateDiff: &rpcv8.StateDiff{
			DeprecatedDeclaredClasses: update.StateDiff.DeclaredV0Classes,
			DeclaredClasses:           declaredClasses,
			ReplacedClasses:           replacedClasses,
			Nonces:                    nonces,
			StorageDiffs:              storageDiffs,
			DeployedContracts:         deployedContracts,
		},
	}, nil
}
