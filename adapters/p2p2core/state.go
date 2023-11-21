package p2p2core

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptStateDiff(s *spec.StateDiff, classes []spec.Class) *core.StateDiff {
	var (
		declaredV0Classes []*felt.Felt
		declaredV1Classes []core.DeclaredV1Class
	)

	for _, class := range classes {
		switch v := class.Class.(type) {
		case *spec.Cairo0Class:
		case *spec.Cairo1Class:
		}
	}

	storageDiffs := make(map[felt.Felt][]core.StorageDiff)
	nonces := make(map[felt.Felt]*felt.Felt)
	for _, diff := range s.ContractDiffs {
		address := AdaptAddress(diff.Address)

		nonces[*address] = AdaptFelt(diff.Nonce)
		storageDiffs[*address] = utils.Map(diff.Values, adaptStoredValue)
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: utils.Map(s.DeployedContracts, adaptAddrToClassHash),
		DeclaredV0Classes: declaredV0Classes,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   utils.Map(s.ReplacedClasses, adaptAddrToClassHash),
	}
}

func adaptStoredValue(v *spec.ContractStoredValue) core.StorageDiff {
	return core.StorageDiff{
		Key:   AdaptFelt(v.Key),
		Value: AdaptFelt(v.Value),
	}
}

func adaptAddrToClassHash(addrToClassHash *spec.StateDiff_ContractAddrToClassHash) core.AddressClassHashPair {
	return core.AddressClassHashPair{
		Address:   AdaptFelt(addrToClassHash.ContractAddr),
		ClassHash: AdaptFelt(addrToClassHash.ClassHash),
	}
}
