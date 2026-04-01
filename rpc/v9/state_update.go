package rpcv9

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

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

type StorageDiff struct {
	Address        felt.Felt `json:"address"`
	StorageEntries []Entry   `json:"storage_entries"`
}

type Entry struct {
	Key   felt.Felt `json:"key"`
	Value felt.Felt `json:"value"`
}

type Nonce struct {
	ContractAddress felt.Felt `json:"contract_address"`
	Nonce           felt.Felt `json:"nonce"`
}

type DeployedContract struct {
	Address   felt.Felt `json:"address"`
	ClassHash felt.Felt `json:"class_hash"`
}

type DeclaredClass struct {
	ClassHash         felt.Felt `json:"class_hash"`
	CompiledClassHash felt.Felt `json:"compiled_class_hash"`
}

type ReplacedClass struct {
	ContractAddress felt.Felt `json:"contract_address"`
	ClassHash       felt.Felt `json:"class_hash"`
}

/****************************************************
		StateUpdate Handlers
*****************************************************/

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L136
func (h *Handler) StateUpdate(id *BlockID) (StateUpdate, *jsonrpc.Error) {
	update, err := h.stateUpdateByID(id)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return StateUpdate{}, rpccore.ErrBlockNotFound
		}
		return StateUpdate{}, rpccore.ErrInternal.CloneWithData(err)
	}

	index := 0
	nonces := make([]Nonce, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces[index] = Nonce{ContractAddress: addr, Nonce: *nonce}
		index++
	}

	index = 0
	storageDiffs := make([]StorageDiff, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]Entry, len(diffs))
		entryIdx := 0
		for key, value := range diffs {
			entries[entryIdx] = Entry{
				Key:   key,
				Value: *value,
			}
			entryIdx++
		}

		storageDiffs[index] = StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		}
		index++
	}

	index = 0
	deployedContracts := make([]DeployedContract, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		deployedContracts[index] = DeployedContract{
			Address:   addr,
			ClassHash: *classHash,
		}
		index++
	}

	index = 0
	declaredClasses := make([]DeclaredClass, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses[index] = DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		}
		index++
	}

	index = 0
	replacedClasses := make([]ReplacedClass, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		replacedClasses[index] = ReplacedClass{
			ClassHash:       *classHash,
			ContractAddress: addr,
		}
		index++
	}

	return StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   nilToZero(update.OldRoot),
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

func (h *Handler) stateUpdateByID(id *BlockID) (*core.StateUpdate, error) {
	switch id.Type() {
	case latest:
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			return nil, heightErr
		} else {
			return h.bcReader.StateUpdateByNumber(height)
		}
	case preConfirmed:
		pending, err := h.syncReader.PreConfirmed()
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
