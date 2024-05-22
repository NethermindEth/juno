package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptContractDiff(addr, nonce *felt.Felt, diff map[felt.Felt]*felt.Felt) *spec.ContractDiff {
	return &spec.ContractDiff{
		Address:    AdaptAddress(addr),
		Nonce:      AdaptFelt(nonce),
		ClassHash:  nil, // This will need to be set if deployed_contracts and replaced_classes are removed from StateDiff
		IsReplaced: nil,
		Values:     AdaptStorageDiff(diff),
		Domain:     0,
	}
}

func AdaptStorageDiff(diff map[felt.Felt]*felt.Felt) []*spec.ContractStoredValue {
	return utils.ToSlice(diff, func(key felt.Felt, value *felt.Felt) *spec.ContractStoredValue {
		return &spec.ContractStoredValue{
			Key:   AdaptFelt(&key),
			Value: AdaptFelt(value),
		}
	})
}

//func AdaptAddressClassHashPair(address felt.Felt, classHash *felt.Felt) *spec.StateDiff_ContractAddrToClassHash {
//	return &spec.StateDiff_ContractAddrToClassHash{
//		ContractAddr: AdaptAddress(&address),
//		ClassHash:    AdaptHash(classHash),
//	}
//}
