package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptStateDiff(addr, classHash, nonce *felt.Felt, diff map[felt.Felt]*felt.Felt) *spec.StateDiff_ContractDiff {
	return &spec.StateDiff_ContractDiff{
		Address:   AdaptAddress(addr),
		Nonce:     AdaptFelt(nonce),
		ClassHash: AdaptFelt(classHash),
		Values:    AdaptStorageDiff(diff),
	}
}

func AdaptStorageDiff(diff map[felt.Felt]*felt.Felt) []*spec.ContractStoredValue {
	result := make([]*spec.ContractStoredValue, len(diff))
	for key, value := range diff {
		result = append(result, &spec.ContractStoredValue{
			Key:   AdaptFelt(&key),
			Value: AdaptFelt(value),
		})
	}
	return result
}

func AdaptAddressClassHashPair(c core.AddressClassHashPair) *spec.StateDiff_ContractAddrToClassHash {
	return &spec.StateDiff_ContractAddrToClassHash{
		ContractAddr: AdaptAddress(c.Address),
		ClassHash:    AdaptHash(c.ClassHash),
	}
}
