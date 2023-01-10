package core

import "github.com/NethermindEth/juno/core/felt"

type StateUpdate struct {
	BlockHash *felt.Felt
	NewRoot   *felt.Felt
	OldRoot   *felt.Felt

	StateDiff struct {
		StorageDiffs map[felt.Felt][]struct {
			Key   *felt.Felt
			Value *felt.Felt
		}

		Nonces            map[felt.Felt]*felt.Felt
		DeployedContracts []struct {
			Address   *felt.Felt
			ClassHash *felt.Felt
		}
		DeclaredContracts []*felt.Felt
	}
}
