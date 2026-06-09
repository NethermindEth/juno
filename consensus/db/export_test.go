package db

import (
	"github.com/NethermindEth/juno/consensus/starknet"
)

// ForcePendingCleanup primes the store so the next prune flush triggers cleanup.
func ForcePendingCleanup(
	walStore TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address],
) {
	walStore.(*tendermintWALStore[starknet.Value, starknet.Hash, starknet.Address]).
		pruneRecordsSinceCleanup = cleanupPruneRecordInterval - 1
}
