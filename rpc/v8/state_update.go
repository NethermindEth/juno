package rpcv8

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L1416
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash,omitempty"`
	NewRoot   *felt.Felt `json:"new_root,omitempty"`
	OldRoot   *felt.Felt `json:"old_root"`
	StateDiff *StateDiff `json:"state_diff"`
}

type StateDiff struct {
	StorageDiffs              []StorageDiff      `json:"storage_diffs"`
	Nonces                    []Nonce            `json:"nonces"`
	DeployedContracts         []DeployedContract `json:"deployed_contracts"`
	DeprecatedDeclaredClasses []*felt.Felt       `json:"deprecated_declared_classes"`
	DeclaredClasses           []DeclaredClass    `json:"declared_classes"`
	ReplacedClasses           []ReplacedClass    `json:"replaced_classes"`
}

type Nonce struct {
	ContractAddress felt.Felt `json:"contract_address"`
	Nonce           felt.Felt `json:"nonce"`
}

type StorageDiff struct {
	Address        felt.Felt `json:"address"`
	StorageEntries []Entry   `json:"storage_entries"`
}

type Entry struct {
	Key   felt.Felt `json:"key"`
	Value felt.Felt `json:"value"`
}

type DeployedContract struct {
	Address   felt.Felt `json:"address"`
	ClassHash felt.Felt `json:"class_hash"`
}

type ReplacedClass struct {
	ContractAddress felt.Felt `json:"contract_address"`
	ClassHash       felt.Felt `json:"class_hash"`
}

type DeclaredClass struct {
	ClassHash         felt.Felt `json:"class_hash"`
	CompiledClassHash felt.Felt `json:"compiled_class_hash"`
}

/****************************************************
		StateUpdate Handlers
*****************************************************/

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.8.1/api/starknet_api_openrpc.json#L136
func (h *Handler) StateUpdate(id BlockID) (*StateUpdate, *jsonrpc.Error) {
	var update *core.StateUpdate
	var err error
	if id.IsLatest() {
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			err = heightErr
		} else {
			update, err = h.bcReader.StateUpdateByNumber(height)
		}
	} else if id.IsPending() {
		//nolint:staticcheck // Necessary for v8
		var pending *core.Pending
		pending, err = h.Pending()
		if err == nil {
			update = pending.GetStateUpdate()
		}
	} else if id.IsHash() {
		update, err = h.bcReader.StateUpdateByHash(id.Hash())
	} else {
		update, err = h.bcReader.StateUpdateByNumber(id.Number())
	}
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	nonces := make([]Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, Nonce{ContractAddress: addr, Nonce: *nonce})
	}

	storageDiffs := make([]StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]Entry, 0, len(diffs))
		for key, value := range diffs {
			entries = append(entries, Entry{
				Key:   key,
				Value: *value,
			})
		}

		storageDiffs = append(storageDiffs, StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		})
	}

	deployedContracts := make([]DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, DeployedContract{
			Address:   addr,
			ClassHash: *classHash,
		})
	}

	declaredClasses := make([]DeclaredClass, 0, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses = append(declaredClasses, DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		})
	}

	replacedClasses := make([]ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		replacedClasses = append(replacedClasses, ReplacedClass{
			ClassHash:       *classHash,
			ContractAddress: addr,
		})
	}

	return &StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   update.OldRoot,
		NewRoot:   update.NewRoot,
		StateDiff: &StateDiff{
			DeprecatedDeclaredClasses: update.StateDiff.DeclaredV0Classes,
			DeclaredClasses:           declaredClasses,
			ReplacedClasses:           replacedClasses,
			Nonces:                    nonces,
			StorageDiffs:              storageDiffs,
			DeployedContracts:         deployedContracts,
		},
	}, nil
}
