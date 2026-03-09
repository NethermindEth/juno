package rpcv10

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

// https://github.com/starkware-libs/starknet-specs/blob/8016dd08ed7cd220168db16f24c8a6827ab88317/api/starknet_api_openrpc.json#L909 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash,omitempty"`
	NewRoot   *felt.Felt `json:"new_root,omitempty"`
	OldRoot   *felt.Felt `json:"old_root,omitempty"`
	StateDiff *StateDiff `json:"state_diff"`
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

type StateDiff struct {
	StorageDiffs              []StorageDiff           `json:"storage_diffs"`
	Nonces                    []Nonce                 `json:"nonces"`
	DeployedContracts         []DeployedContract      `json:"deployed_contracts"`
	DeprecatedDeclaredClasses []*felt.Felt            `json:"deprecated_declared_classes"`
	DeclaredClasses           []DeclaredClass         `json:"declared_classes"`
	ReplacedClasses           []ReplacedClass         `json:"replaced_classes"`
	MigratedCompiledClasses   []MigratedCompiledClass `json:"migrated_compiled_classes"`
}

type MigratedCompiledClass struct {
	ClassHash         felt.SierraClassHash `json:"class_hash"`
	CompiledClassHash felt.CasmClassHash   `json:"compiled_class_hash"`
}

/****************************************************
		StateUpdate Handlers
*****************************************************/

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L136 //nolint:lll
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
func (h *Handler) StateUpdate(id *BlockID, contractAddresses AddressList) (StateUpdate, *jsonrpc.Error) {
	update, err := h.stateUpdateByID(id)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
			return StateUpdate{}, rpccore.ErrBlockNotFound
		}
		return StateUpdate{}, rpccore.ErrInternal.CloneWithData(err)
	}

	nonces := make([]Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		if contractAddresses.Contains((*felt.Address)(&addr)) {
			nonces = append(nonces, Nonce{
				ContractAddress: addr,
				Nonce:           *nonce,
			})
		}
	}

	storageDiffs := make([]StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		if !contractAddresses.Contains((*felt.Address)(&addr)) {
			continue
		}
		entries := make([]Entry, len(diffs))
		entryIdx := 0
		for key, value := range diffs {
			entries[entryIdx] = Entry{
				Key:   key,
				Value: *value,
			}
			entryIdx++
		}
		storageDiffs = append(storageDiffs, StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		})
	}

	deployedContracts := make([]DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		if contractAddresses.Contains((*felt.Address)(&addr)) {
			deployedContracts = append(deployedContracts, DeployedContract{
				Address:   addr,
				ClassHash: *classHash,
			})
		}
	}

	index := 0
	declaredClasses := make([]DeclaredClass, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses[index] = DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		}
		index++
	}

	replacedClasses := make([]ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		if contractAddresses.Contains((*felt.Address)(&addr)) {
			replacedClasses = append(replacedClasses, ReplacedClass{
				ClassHash:       *classHash,
				ContractAddress: addr,
			})
		}
	}

	index = 0
	migratedCompiledClasses := make([]MigratedCompiledClass, len(update.StateDiff.MigratedClasses))
	for classHash, casmClassHash := range update.StateDiff.MigratedClasses {
		migratedCompiledClasses[index] = MigratedCompiledClass{
			ClassHash:         classHash,
			CompiledClassHash: casmClassHash,
		}
		index++
	}

	return StateUpdate{
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
			MigratedCompiledClasses:   migratedCompiledClasses,
		},
	}, nil
}

func (h *Handler) stateUpdateByID(id *BlockID) (*core.StateUpdate, error) {
	switch {
	case id.IsLatest():
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			return nil, heightErr
		} else {
			return h.bcReader.StateUpdateByNumber(height)
		}
	case id.IsPreConfirmed():
		var pending core.PendingData
		pending, err := h.PendingData()
		if err != nil {
			return nil, err
		}
		return pending.GetStateUpdate(), nil
	case id.IsHash():
		return h.bcReader.StateUpdateByHash(id.Hash())
	case id.IsNumber():
		return h.bcReader.StateUpdateByNumber(id.Number())
	case id.IsL1Accepted():
		l1Head, err := h.bcReader.L1Head()
		if err != nil {
			return nil, err
		}
		return h.bcReader.StateUpdateByNumber(l1Head.BlockNumber)
	default:
		panic("unknown block type id")
	}
}
