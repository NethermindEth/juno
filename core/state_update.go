package core

import "github.com/NethermindEth/juno/core/felt"

type KVPair struct {
	Key   *felt.Felt
	Value *felt.Felt
}

type StateUpdate struct {
	BlockHash *felt.Felt
	NewRoot   *felt.Felt
	OldRoot   *felt.Felt

	StateDiff struct {
		StorageDiffs map[string][]KVPair

		Nonces            map[string]*felt.Felt
		DeployedContracts []struct {
			Address   *felt.Felt
			ClassHash *felt.Felt
		}
		DeclaredContracts []*felt.Felt
	}
}
