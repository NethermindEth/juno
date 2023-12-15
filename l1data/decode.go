package l1data

import (
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

const (
	classInfoBit           = 128
	numStorageUpdatesWidth = 64
)

type State interface {
	ContractIsDeployed(address *felt.Felt) (bool, error)
}

// Decode011Diff decodes an L1-formatted state diff. The result of decoding an improperly encoded state diff is undefined.
func Decode011Diff(encodedDiff []*big.Int, state State) (*core.StateDiff, error) {
	offset := uint64(0)
	numContractUpdates := encodedDiff[offset].Uint64()
	offset++
	nonces := make(map[felt.Felt]*felt.Felt)
	deployedContracts := make(map[felt.Felt]*felt.Felt)
	replacedClasses := make(map[felt.Felt]*felt.Felt)
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for i := uint64(0); i < numContractUpdates; i++ {
		address := new(felt.Felt).SetBigInt(encodedDiff[offset])
		offset++

		summary := encodedDiff[offset]
		offset++

		numStorageUpdates := summary.Uint64()
		classInfoFlag := summary.Bit(classInfoBit)
		nonces[*address] = new(felt.Felt).SetUint64(summary.Rsh(summary, numStorageUpdatesWidth).Uint64())

		if classInfoFlag == 1 {
			classHash := new(felt.Felt).SetBigInt(encodedDiff[offset])
			offset++
			if alreadyDeployed, err := state.ContractIsDeployed(address); err != nil {
				return nil, fmt.Errorf("determine if contract %s is already deployed: %w", address.Text(felt.Base16), err)
			} else if alreadyDeployed {
				replacedClasses[*address] = classHash
			} else {
				deployedContracts[*address] = classHash
			}
		}

		if numStorageUpdates > 0 {
			storageDiffs[*address] = make(map[felt.Felt]*felt.Felt, numStorageUpdates)
			for j := uint64(0); j < numStorageUpdates; j++ {
				key := new(felt.Felt).SetBigInt(encodedDiff[offset])
				offset++
				value := new(felt.Felt).SetBigInt(encodedDiff[offset])
				offset++
				storageDiffs[*address][*key] = value
			}
		}
	}

	numDeclaredClasses := encodedDiff[offset].Uint64()
	offset++
	declaredClasses := make(map[felt.Felt]*felt.Felt, numDeclaredClasses)
	for i := uint64(0); i < numDeclaredClasses; i++ {
		classHash := new(felt.Felt).SetBigInt(encodedDiff[offset])
		offset++
		compiledClassHash := new(felt.Felt).SetBigInt(encodedDiff[offset])
		offset++
		declaredClasses[*classHash] = compiledClassHash
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: deployedContracts,
		DeclaredV1Classes: declaredClasses,
		ReplacedClasses:   replacedClasses,
	}, nil
}

// DecodePre011Diff decodes an L1-formatted state diff. The result of decoding an improperly encoded state diff is undefined.
// If withConstructorArgs is true, constructor arguments are assumed to be present in encodedDiff.
func DecodePre011Diff(encodedDiff []*big.Int, withConstructorArgs bool) *core.StateDiff {
	// Parse contract deployments.
	numDeploymentsCells := encodedDiff[0].Uint64()
	offset := uint64(1)
	deployedContracts := make(map[felt.Felt]*felt.Felt, 0)
	for offset-1 < numDeploymentsCells {
		address := new(felt.Felt).SetBigInt(encodedDiff[offset])
		offset++
		classHash := new(felt.Felt).SetBigInt(encodedDiff[offset])
		offset++
		deployedContracts[*address] = classHash

		if withConstructorArgs {
			constructorArgsLen := encodedDiff[offset].Uint64()
			offset++
			offset += constructorArgsLen
		}
	}

	// Parse contract storage updates.
	updatesLen := encodedDiff[offset].Uint64()
	offset++
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	nonces := make(map[felt.Felt]*felt.Felt)
	for i := uint64(0); i < updatesLen; i++ {
		address := new(felt.Felt).SetBigInt(encodedDiff[offset])
		offset++
		numUpdates := encodedDiff[offset].Uint64()
		nonces[*address] = new(felt.Felt).SetBigInt(new(big.Int).Rsh(encodedDiff[offset], numStorageUpdatesWidth))
		offset++

		storageDiffs[*address] = make(map[felt.Felt]*felt.Felt, numUpdates)
		for j := uint64(0); j < numUpdates; j++ {
			key := new(felt.Felt).SetBigInt(encodedDiff[offset])
			offset++
			value := new(felt.Felt).SetBigInt(encodedDiff[offset])
			offset++
			storageDiffs[*address][*key] = value
		}
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: deployedContracts,
		DeclaredV0Classes: nil,
		DeclaredV1Classes: nil,
		ReplacedClasses:   nil,
	}
}
