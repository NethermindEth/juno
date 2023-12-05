package builder

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type StateDiff struct {
	StorageDiffs      map[felt.Felt]map[felt.Felt]*felt.Felt
	Nonces            map[felt.Felt]*felt.Felt
	DeployedContracts map[felt.Felt]*felt.Felt
	DeclaredV0Classes []*felt.Felt
	DeclaredV1Classes map[felt.Felt]*felt.Felt
	ReplacedClasses   map[felt.Felt]*felt.Felt
	Classes           map[felt.Felt]core.Class
}

func AdaptCoreDiffToExecutionDiff(sd *core.StateDiff, classes map[felt.Felt]core.Class) *StateDiff {
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(sd.StorageDiffs))
	for addr, diff := range sd.StorageDiffs {
		newStorageDiffs := make(map[felt.Felt]*felt.Felt, len(diff))
		for _, kv := range diff {
			newStorageDiffs[*kv.Key] = kv.Value
		}
		storageDiffs[addr] = newStorageDiffs
	}

	nonces := make(map[felt.Felt]*felt.Felt, len(sd.Nonces))
	for addr, nonce := range sd.Nonces {
		nonces[addr] = nonce
	}

	deployedContracts := make(map[felt.Felt]*felt.Felt, len(sd.DeployedContracts))
	for _, deployedContract := range sd.DeployedContracts {
		deployedContracts[*deployedContract.Address] = deployedContract.ClassHash
	}

	declaredV1Classes := make(map[felt.Felt]*felt.Felt, len(sd.DeclaredV1Classes))
	for _, declaredV1Class := range sd.DeclaredV1Classes {
		declaredV1Classes[*declaredV1Class.ClassHash] = declaredV1Class.CompiledClassHash
	}

	replacedClasses := make(map[felt.Felt]*felt.Felt, len(sd.ReplacedClasses))
	for _, replacedClass := range sd.ReplacedClasses {
		replacedClasses[*replacedClass.Address] = replacedClass.ClassHash
	}

	return &StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: deployedContracts,
		DeclaredV0Classes: sd.DeclaredV0Classes,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   replacedClasses,
		Classes:           classes,
	}
}

func AdaptStateDiffToCoreDiff(sd *StateDiff) (*core.StateDiff, map[felt.Felt]core.Class) {
	// TODO don't create an empty diff. We should create separate maps/arrays
	// that have their capacity set to the capacity of the corresponding object in `sd`,
	// and then build the coreDiff at the end.
	coreDiff := core.EmptyStateDiff()

	for addr, diffs := range sd.StorageDiffs {
		storageDiffs := make([]core.StorageDiff, 0, len(diffs))
		for key, value := range diffs {
			keyCopy := key
			storageDiffs = append(storageDiffs, core.StorageDiff{
				Key:   &keyCopy,
				Value: value,
			})
		}
		coreDiff.StorageDiffs[addr] = storageDiffs
	}

	for addr, nonce := range sd.Nonces {
		coreDiff.Nonces[addr] = nonce
	}

	for addr, classHash := range sd.DeployedContracts {
		addrCopy := addr
		coreDiff.DeployedContracts = append(coreDiff.DeployedContracts, core.AddressClassHashPair{
			Address:   &addrCopy,
			ClassHash: classHash,
		})
	}

	coreDiff.DeclaredV0Classes = sd.DeclaredV0Classes

	for classHash, compiledClassHash := range sd.DeclaredV1Classes {
		classHashCopy := classHash
		coreDiff.DeclaredV1Classes = append(coreDiff.DeclaredV1Classes, core.DeclaredV1Class{
			ClassHash:         &classHashCopy,
			CompiledClassHash: compiledClassHash,
		})
	}

	for addr, classHash := range sd.ReplacedClasses {
		addrCopy := addr
		coreDiff.ReplacedClasses = append(coreDiff.ReplacedClasses, core.AddressClassHashPair{
			Address:   &addrCopy,
			ClassHash: classHash,
		})
	}

	return coreDiff, sd.Classes
}
