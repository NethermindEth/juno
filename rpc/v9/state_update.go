package rpcv9

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
)

/****************************************************
		StateUpdate Handlers
*****************************************************/

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L136
func (h *Handler) StateUpdate(id *BlockID) (rpcv6.StateUpdate, *jsonrpc.Error) {
	update, err := h.stateUpdateByID(id)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
			return rpcv6.StateUpdate{}, rpccore.ErrBlockNotFound
		}
		return rpcv6.StateUpdate{}, rpccore.ErrInternal.CloneWithData(err)
	}

	index := 0
	nonces := make([]rpcv6.Nonce, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces[index] = rpcv6.Nonce{ContractAddress: addr, Nonce: *nonce}
		index++
	}

	index = 0
	storageDiffs := make([]rpcv6.StorageDiff, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]rpcv6.Entry, len(diffs))
		entryIdx := 0
		for key, value := range diffs {
			entries[entryIdx] = rpcv6.Entry{
				Key:   key,
				Value: *value,
			}
			entryIdx++
		}

		storageDiffs[index] = rpcv6.StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		}
		index++
	}

	index = 0
	deployedContracts := make([]rpcv6.DeployedContract, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		deployedContracts[index] = rpcv6.DeployedContract{
			Address:   addr,
			ClassHash: *classHash,
		}
		index++
	}

	index = 0
	declaredClasses := make([]rpcv6.DeclaredClass, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses[index] = rpcv6.DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		}
		index++
	}

	index = 0
	replacedClasses := make([]rpcv6.ReplacedClass, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		replacedClasses[index] = rpcv6.ReplacedClass{
			ClassHash:       *classHash,
			ContractAddress: addr,
		}
		index++
	}

	return rpcv6.StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   nilToZero(update.OldRoot),
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

func (h *Handler) stateUpdateByID(id *BlockID) (*core.StateUpdate, error) {
	switch id.Type() {
	case latest:
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			return nil, heightErr
		} else {
			return h.bcReader.StateUpdateByNumber(height)
		}
	case preConfirmed:
		var pending core.PendingData
		pending, err := h.PendingData()
		if err != nil {
			return nil, err
		}
		return pending.GetStateUpdate(), nil
	case hash:
		return h.bcReader.StateUpdateByHash(id.Hash())
	case number:
		return h.bcReader.StateUpdateByNumber(id.Number())
	case l1Accepted:
		l1Head, err := h.bcReader.L1Head()
		if err != nil {
			return nil, err
		}
		return h.bcReader.StateUpdateByNumber(l1Head.BlockNumber)
	default:
		panic("unknown block type id")
	}
}
