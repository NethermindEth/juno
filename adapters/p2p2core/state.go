package p2p2core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptStateDiff(reader core.StateReader, contractDiffs []*spec.ContractDiff, classes []*spec.Class) *core.StateDiff {
	var (
		declaredV0Classes []*felt.Felt
		declaredV1Classes = make(map[felt.Felt]*felt.Felt)
	)

	for _, class := range utils.Map(classes, AdaptClass) {
		h, err := class.Hash()
		if err != nil {
			panic(fmt.Errorf("unexpected error: %v when calculating class hash", err))
		}
		switch c := class.(type) {
		case *core.Cairo0Class:
			declaredV0Classes = append(declaredV0Classes, h)
		case *core.Cairo1Class:
			declaredV1Classes[*h] = c.Compiled.Hash()
			// todo add type?
		}
	}

	var deployedContracts, replacedClasses []addrToClassHash
	// addr -> {key -> value, ...}
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	nonces := make(map[felt.Felt]*felt.Felt)
	for _, diff := range contractDiffs {
		address := AdaptAddress(diff.Address)

		if diff.Nonce != nil {
			nonces[*address] = AdaptFelt(diff.Nonce)
		}
		if diff.Values != nil {
			storageDiffs[*address] = utils.ToMap(diff.Values, adaptStoredValue)
		}

		if diff.ClassHash != nil {
			addrToClsHash := addrToClassHash{
				addr:      diff.Address,
				classHash: diff.ClassHash,
			}

			var stateClassHash *felt.Felt
			if reader == nil {
				// zero block
				stateClassHash = &felt.Zero
			} else {
				var err error
				stateClassHash, err = reader.ContractClassHash(address)
				if err != nil {
					if errors.Is(err, db.ErrKeyNotFound) {
						stateClassHash = &felt.Zero
					} else {
						panic(err)
					}
				}
			}

			if !stateClassHash.IsZero() {
				replacedClasses = append(replacedClasses, addrToClsHash)
			} else {
				deployedContracts = append(deployedContracts, addrToClsHash)
			}
		}
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: utils.ToMap(deployedContracts, adaptAddrToClassHash),
		DeclaredV0Classes: declaredV0Classes,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   utils.ToMap(replacedClasses, adaptAddrToClassHash),
	}
}

func adaptStoredValue(v *spec.ContractStoredValue) (felt.Felt, *felt.Felt) {
	return *AdaptFelt(v.Key), AdaptFelt(v.Value)
}

type addrToClassHash struct {
	addr      *spec.Address
	classHash *spec.Hash
}

// todo rename
func adaptAddrToClassHash(v addrToClassHash) (felt.Felt, *felt.Felt) {
	return *AdaptAddress(v.addr), AdaptHash(v.classHash)
}
