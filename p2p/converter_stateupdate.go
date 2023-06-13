package p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/grpcclient"
)

func coreStateUpdateToProtobufStateUpdate(corestateupdate *core.StateUpdate) *grpcclient.StateDiffs_BlockStateUpdateWithHash {
	contractdiffs := map[felt.Felt]*grpcclient.BlockStateUpdate_ContractDiff{}
	i := 0
	for key, diffs := range corestateupdate.StateDiff.StorageDiffs {

		storagediff := make([]*grpcclient.BlockStateUpdate_StorageDiff, len(diffs))
		for i2, diff := range diffs {
			storagediff[i2] = &grpcclient.BlockStateUpdate_StorageDiff{
				Key:   feltToFieldElement(diff.Key),
				Value: feltToFieldElement(diff.Value),
			}
		}

		contractdiffs[key] = &grpcclient.BlockStateUpdate_ContractDiff{
			ContractAddress: feltToFieldElement(&key),
			StorageDiffs:    storagediff,
		}

		i += 1
	}

	for k, nonce := range corestateupdate.StateDiff.Nonces {
		if _, ok := contractdiffs[k]; !ok {
			contractdiffs[k] = &grpcclient.BlockStateUpdate_ContractDiff{
				ContractAddress: feltToFieldElement(&k),
			}
		}
		contractdiffs[k].Nonce = feltToFieldElement(nonce)
	}

	contractdiffarr := make([]*grpcclient.BlockStateUpdate_ContractDiff, 0)
	for _, diff := range contractdiffs {
		contractdiffarr = append(contractdiffarr, diff)
	}

	deployedcontracts := make([]*grpcclient.BlockStateUpdate_DeployedContract, len(corestateupdate.StateDiff.DeployedContracts))
	for i, contract := range corestateupdate.StateDiff.DeployedContracts {
		deployedcontracts[i] = &grpcclient.BlockStateUpdate_DeployedContract{
			ContractAddress:   feltToFieldElement(contract.Address),
			ContractClassHash: feltToFieldElement(contract.ClassHash),
		}
	}

	declaredv1classes := make([]*grpcclient.BlockStateUpdate_DeclaredV1Class, len(corestateupdate.StateDiff.DeclaredV1Classes))
	for i, contract := range corestateupdate.StateDiff.DeclaredV1Classes {
		declaredv1classes[i] = &grpcclient.BlockStateUpdate_DeclaredV1Class{
			ClassHash:         feltToFieldElement(contract.ClassHash),
			CompiledClassHash: feltToFieldElement(contract.CompiledClassHash),
		}
	}

	replacedclasses := make([]*grpcclient.BlockStateUpdate_ReplacedClasses, len(corestateupdate.StateDiff.ReplacedClasses))
	for i, contract := range corestateupdate.StateDiff.ReplacedClasses {
		replacedclasses[i] = &grpcclient.BlockStateUpdate_ReplacedClasses{
			ContractAddress:   feltToFieldElement(contract.Address),
			ContractClassHash: feltToFieldElement(contract.ClassHash),
		}
	}

	stateupdate := &grpcclient.BlockStateUpdate{
		ContractDiffs:               contractdiffarr,
		DeployedContracts:           deployedcontracts,
		DeclaredContractClassHashes: feltsToFieldElements(corestateupdate.StateDiff.DeclaredV0Classes),
		DeclaredV1Classes:           declaredv1classes,
		ReplacedClasses:             replacedclasses,
	}

	return &grpcclient.StateDiffs_BlockStateUpdateWithHash{
		StateUpdate: stateupdate,
	}
}

func protobufStateUpdateToCoreStateUpdate(pbStateUpdate *grpcclient.StateDiffs_BlockStateUpdateWithHash) *core.StateUpdate {
	stateUpdate := pbStateUpdate.StateUpdate

	coreStateUpdate := &core.StateUpdate{
		StateDiff: &core.StateDiff{},
	}

	// Invert ContractDiffs
	contractDiffs := stateUpdate.ContractDiffs
	storageDiffs := make(map[felt.Felt][]core.StorageDiff)
	nonces := make(map[felt.Felt]*felt.Felt)

	for _, contractDiff := range contractDiffs {
		contractAddress := fieldElementToFelt(contractDiff.ContractAddress)
		if contractDiff.Nonce != nil {
			nonces[*contractAddress] = fieldElementToFelt(contractDiff.Nonce)
		}

		if contractDiff.StorageDiffs != nil {
			storageDiff := make([]core.StorageDiff, len(contractDiff.StorageDiffs))
			for i, diff := range contractDiff.StorageDiffs {
				storageDiff[i] = core.StorageDiff{
					Key:   fieldElementToFelt(diff.Key),
					Value: fieldElementToFelt(diff.Value),
				}
			}

			storageDiffs[*contractAddress] = storageDiff
		}
	}

	coreStateUpdate.StateDiff.StorageDiffs = storageDiffs
	coreStateUpdate.StateDiff.Nonces = nonces

	// Invert DeployedContracts
	deployedContracts := stateUpdate.DeployedContracts
	convertedDeployedContracts := make([]core.DeployedContract, len(deployedContracts))
	for i, contract := range deployedContracts {
		convertedDeployedContracts[i] = core.DeployedContract{
			Address:   fieldElementToFelt(contract.ContractAddress),
			ClassHash: fieldElementToFelt(contract.ContractClassHash),
		}
	}
	coreStateUpdate.StateDiff.DeployedContracts = convertedDeployedContracts

	// Invert DeclaredV1Classes
	declaredV1Classes := stateUpdate.DeclaredV1Classes
	convertedDeclaredV1Classes := make([]core.DeclaredV1Class, len(declaredV1Classes))
	for i, contract := range declaredV1Classes {
		convertedDeclaredV1Classes[i] = core.DeclaredV1Class{
			ClassHash:         fieldElementToFelt(contract.ClassHash),
			CompiledClassHash: fieldElementToFelt(contract.CompiledClassHash),
		}
	}
	coreStateUpdate.StateDiff.DeclaredV1Classes = convertedDeclaredV1Classes

	// Invert ReplacedClasses
	replacedClasses := stateUpdate.ReplacedClasses
	convertedReplacedClasses := make([]core.ReplacedClass, len(replacedClasses))
	for i, contract := range replacedClasses {
		convertedReplacedClasses[i] = core.ReplacedClass{
			Address:   fieldElementToFelt(contract.ContractAddress),
			ClassHash: fieldElementToFelt(contract.ContractClassHash),
		}
	}
	coreStateUpdate.StateDiff.ReplacedClasses = convertedReplacedClasses
	coreStateUpdate.StateDiff.DeclaredV0Classes = fieldElementsToFelts(stateUpdate.DeclaredContractClassHashes)

	return coreStateUpdate
}
