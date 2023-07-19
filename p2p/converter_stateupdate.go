package p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/p2pproto"
)

func coreStateUpdateToProtobufStateUpdate(corestateupdate *core.StateUpdate) *p2pproto.StateDiffs_BlockStateUpdateWithHash {
	contractdiffs := map[felt.Felt]*p2pproto.BlockStateUpdate_ContractDiff{}
	for key, diffs := range corestateupdate.StateDiff.StorageDiffs {
		contractdiffs[key] = &p2pproto.BlockStateUpdate_ContractDiff{
			ContractAddress: feltToFieldElement(&key),
			StorageDiffs:    MapValueWithReflection[[]*p2pproto.BlockStateUpdate_StorageDiff](diffs),
		}
	}

	for k, nonce := range corestateupdate.StateDiff.Nonces {
		if _, ok := contractdiffs[k]; !ok {
			contractdiffs[k] = &p2pproto.BlockStateUpdate_ContractDiff{
				ContractAddress: feltToFieldElement(&k),
			}
		}
		contractdiffs[k].Nonce = feltToFieldElement(nonce)
	}

	contractdiffarr := make([]*p2pproto.BlockStateUpdate_ContractDiff, 0)
	for _, diff := range contractdiffs {
		contractdiffarr = append(contractdiffarr, diff)
	}

	deployedcontracts := protoMapArray(corestateupdate.StateDiff.DeployedContracts,
		func(contract core.DeployedContract) *p2pproto.BlockStateUpdate_DeployedContract {
			return &p2pproto.BlockStateUpdate_DeployedContract{
				ContractAddress:   feltToFieldElement(contract.Address),
				ContractClassHash: feltToFieldElement(contract.ClassHash),
			}
		})

	declaredv1classes := protoMapArray(corestateupdate.StateDiff.DeclaredV1Classes,
		func(contract core.DeclaredV1Class) *p2pproto.BlockStateUpdate_DeclaredV1Class {
			return &p2pproto.BlockStateUpdate_DeclaredV1Class{
				ClassHash:         feltToFieldElement(contract.ClassHash),
				CompiledClassHash: feltToFieldElement(contract.CompiledClassHash),
			}
		})

	replacedclasses := protoMapArray(corestateupdate.StateDiff.ReplacedClasses,
		func(contract core.ReplacedClass) *p2pproto.BlockStateUpdate_ReplacedClasses {
			return &p2pproto.BlockStateUpdate_ReplacedClasses{
				ContractAddress:   feltToFieldElement(contract.Address),
				ContractClassHash: feltToFieldElement(contract.ClassHash),
			}
		})

	stateupdate := &p2pproto.BlockStateUpdate{
		ContractDiffs:               contractdiffarr,
		DeployedContracts:           deployedcontracts,
		DeclaredContractClassHashes: feltsToFieldElements(corestateupdate.StateDiff.DeclaredV0Classes),
		DeclaredV1Classes:           declaredv1classes,
		ReplacedClasses:             replacedclasses,
	}

	return &p2pproto.StateDiffs_BlockStateUpdateWithHash{
		StateUpdate: stateupdate,
	}
}

func protobufStateUpdateToCoreStateUpdate(pbStateUpdate *p2pproto.StateDiffs_BlockStateUpdateWithHash) *core.StateUpdate {
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
			storageDiffs[*contractAddress] = MapValueWithReflection[[]core.StorageDiff](contractDiff.StorageDiffs)
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
