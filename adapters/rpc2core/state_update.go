package rpc2core

import (
	"errors"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
)

func AdaptStateUpdate(response *rpcv10.StateUpdate) (*core.StateUpdate, error) {
	switch {
	case response == nil:
		return nil, errors.New("nil state update")
	case response.BlockHash == nil:
		return nil, errors.New("state update is missing block_hash")
	case response.NewRoot == nil:
		return nil, fmt.Errorf("state update for block %s is missing new_root", response.BlockHash)
	case response.OldRoot == nil:
		return nil, fmt.Errorf("state update for block %s is missing old_root", response.BlockHash)
	case response.StateDiff == nil:
		return nil, fmt.Errorf("state update for block %s is missing state_diff", response.BlockHash)
	case slices.Contains(response.StateDiff.DeprecatedDeclaredClasses, nil):
		return nil, fmt.Errorf(
			"state update for block %s has a null deprecated declared class", response.BlockHash,
		)
	}
	return &core.StateUpdate{
		BlockHash: response.BlockHash,
		NewRoot:   response.NewRoot,
		OldRoot:   response.OldRoot,
		StateDiff: adaptStateDiff(response.StateDiff),
	}, nil
}

func adaptStateDiff(diff *rpcv10.StateDiff) *core.StateDiff {
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs))
	for i := range diff.StorageDiffs {
		contractDiff := &diff.StorageDiffs[i]
		entries := make(map[felt.Felt]*felt.Felt, len(contractDiff.StorageEntries))
		for j := range contractDiff.StorageEntries {
			entry := &contractDiff.StorageEntries[j]
			entries[entry.Key] = &entry.Value
		}
		storageDiffs[contractDiff.Address] = entries
	}

	nonces := make(map[felt.Felt]*felt.Felt, len(diff.Nonces))
	for i := range diff.Nonces {
		nonces[diff.Nonces[i].ContractAddress] = &diff.Nonces[i].Nonce
	}

	deployedContracts := make(map[felt.Felt]*felt.Felt, len(diff.DeployedContracts))
	for i := range diff.DeployedContracts {
		deployedContracts[diff.DeployedContracts[i].Address] = &diff.DeployedContracts[i].ClassHash
	}

	declaredV1Classes := make(map[felt.Felt]*felt.Felt, len(diff.DeclaredClasses))
	for i := range diff.DeclaredClasses {
		declaredV1Classes[diff.DeclaredClasses[i].ClassHash] = &diff.DeclaredClasses[i].CompiledClassHash
	}

	replacedClasses := make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses))
	for i := range diff.ReplacedClasses {
		replacedClasses[diff.ReplacedClasses[i].ContractAddress] = &diff.ReplacedClasses[i].ClassHash
	}

	migratedClasses := make(
		map[felt.SierraClassHash]felt.CasmClassHash, len(diff.MigratedCompiledClasses),
	)
	for _, migrated := range diff.MigratedCompiledClasses {
		migratedClasses[migrated.ClassHash] = migrated.CompiledClassHash
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: deployedContracts,
		DeclaredV0Classes: diff.DeprecatedDeclaredClasses,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   replacedClasses,
		MigratedClasses:   migratedClasses,
	}
}
