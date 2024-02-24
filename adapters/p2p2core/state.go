package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptStateDiff(s *spec.StateDiff, classes []*spec.Class) *core.StateDiff {
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
		}
	}

	// addr -> {key -> value, ...}
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	nonces := make(map[felt.Felt]*felt.Felt)
	for _, diff := range s.ContractDiffs {
		address := AdaptAddress(diff.Address)

		if diff.Nonce != nil {
			nonces[*address] = AdaptFelt(diff.Nonce)
		}
		storageDiffs[*address] = utils.ToMap(diff.Values, adaptStoredValue)
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: utils.ToMap(s.DeployedContracts, adaptAddrToClassHash),
		DeclaredV0Classes: declaredV0Classes,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   utils.ToMap(s.ReplacedClasses, adaptAddrToClassHash),
	}
}

func adaptStoredValue(v *spec.ContractStoredValue) (felt.Felt, *felt.Felt) {
	return *AdaptFelt(v.Key), AdaptFelt(v.Value)
}

// todo rename
func adaptAddrToClassHash(v *spec.StateDiff_ContractAddrToClassHash) (felt.Felt, *felt.Felt) {
	return *AdaptAddress(v.ContractAddr), AdaptHash(v.ClassHash)
}
