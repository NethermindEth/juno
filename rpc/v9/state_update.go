package rpcv9

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/sync"
)

/****************************************************
		StateUpdate Handlers
*****************************************************/

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L136
func (h *Handler) StateUpdate(id BlockID) (*rpcv6.StateUpdate, *jsonrpc.Error) {
	update, err := h.stateUpdateByID(id)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, sync.ErrPendingBlockNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	nonces := make([]rpcv6.Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, rpcv6.Nonce{ContractAddress: addr, Nonce: *nonce})
	}

	storageDiffs := make([]rpcv6.StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]rpcv6.Entry, 0, len(diffs))
		for key, value := range diffs {
			entries = append(entries, rpcv6.Entry{
				Key:   key,
				Value: *value,
			})
		}

		storageDiffs = append(storageDiffs, rpcv6.StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		})
	}

	deployedContracts := make([]rpcv6.DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, rpcv6.DeployedContract{
			Address:   addr,
			ClassHash: *classHash,
		})
	}

	declaredClasses := make([]rpcv6.DeclaredClass, 0, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses = append(declaredClasses, rpcv6.DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		})
	}

	replacedClasses := make([]rpcv6.ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		replacedClasses = append(replacedClasses, rpcv6.ReplacedClass{
			ClassHash:       *classHash,
			ContractAddress: addr,
		})
	}

	return &rpcv6.StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   update.OldRoot,
		NewRoot:   update.NewRoot,
		StateDiff: &rpcv6.StateDiff{
			DeprecatedDeclaredClasses: update.StateDiff.DeclaredV0Classes,
			DeclaredClasses:           declaredClasses,
			ReplacedClasses:           replacedClasses,
			Nonces:                    nonces,
			StorageDiffs:              storageDiffs,
			DeployedContracts:         deployedContracts,
		},
	}, nil
}

func (h *Handler) stateUpdateByID(id BlockID) (*core.StateUpdate, error) {
	switch id.Type() {
	case latest:
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			return nil, heightErr
		} else {
			return h.bcReader.StateUpdateByNumber(height)
		}
	case preConfirmed:
		var pending *core.PendingData
		pending, err := h.PendingData()
		if err != nil {
			return nil, err
		}
		return pending.GetStateUpdate(), nil
	case hash:
		return h.bcReader.StateUpdateByHash(id.Hash())
	case number:
		return h.bcReader.StateUpdateByNumber(id.Number())
	default:
		panic("unknown block type id")
	}
}
