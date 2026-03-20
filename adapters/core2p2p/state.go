package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/state"
)

func AdaptContractDiff(addr, nonce, classHash *felt.Felt, storageDiff map[felt.Felt]*felt.Felt) *state.ContractDiff {
	return &state.ContractDiff{
		Address:   AdaptAddress(addr),
		Nonce:     AdaptFelt(nonce),
		ClassHash: AdaptHash(classHash), // This will need to be set if deployed_contracts and replaced_classes are removed from StateDiff
		Values:    AdaptStorageDiff(storageDiff),
		Domain:    0,
	}
}

func AdaptStorageDiff(diff map[felt.Felt]*felt.Felt) []*state.ContractStoredValue {
	return utils.ToSlice(diff, func(key felt.Felt, value *felt.Felt) *state.ContractStoredValue {
		return &state.ContractStoredValue{
			Key:   AdaptFelt(&key),
			Value: AdaptFelt(value),
		}
	})
}
