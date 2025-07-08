package builder

import (
	"maps"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

type BuildState struct {
	Pending           *sync.Pending
	L2GasConsumed     uint64
	RevealedBlockHash *felt.Felt
}

func (b *BuildState) PendingBlock() *core.Block {
	if b.Pending == nil {
		return nil
	}
	return b.Pending.Block
}

func (b *BuildState) ClearPending() error {
	b.L2GasConsumed = 0
	b.Pending = &sync.Pending{}
	b.RevealedBlockHash = nil

	return nil
}

// Assumptions we make to avoid deep copying some fields and types:
// - This function is only called before running `Finish`
// - *felt.Felt is immutable
// - *GasPrice is immutable
// - Signatures and EventsBloom are not set before `Finish` is called
func (b *BuildState) Clone() BuildState {
	return BuildState{
		Pending:           clonePending(b.Pending),
		RevealedBlockHash: b.RevealedBlockHash, // Safe to reuse an immutable value
		L2GasConsumed:     b.L2GasConsumed,     // Value, safe to shallow copy
	}
}

func clonePending(pending *sync.Pending) *sync.Pending {
	return &sync.Pending{
		Block:       cloneBlock(pending.Block),
		StateUpdate: cloneStateUpdate(pending.StateUpdate),
		NewClasses:  maps.Clone(pending.NewClasses),
	}
}

func cloneBlock(block *core.Block) *core.Block {
	return &core.Block{
		Header:       utils.HeapPtr(*block.Header),
		Transactions: slices.Clone(block.Transactions),
		Receipts:     slices.Clone(block.Receipts),
	}
}

func cloneStateUpdate(stateUpdate *core.StateUpdate) *core.StateUpdate {
	return &core.StateUpdate{
		BlockHash: stateUpdate.BlockHash,
		NewRoot:   stateUpdate.NewRoot,
		OldRoot:   stateUpdate.OldRoot,
		StateDiff: cloneStateDiff(stateUpdate.StateDiff),
	}
}

func cloneStateDiff(stateDiff *core.StateDiff) *core.StateDiff {
	return &core.StateDiff{
		StorageDiffs:      cloneStorageDiffs(stateDiff.StorageDiffs),
		Nonces:            maps.Clone(stateDiff.Nonces),
		DeployedContracts: maps.Clone(stateDiff.DeployedContracts),
		DeclaredV0Classes: slices.Clone(stateDiff.DeclaredV0Classes),
		DeclaredV1Classes: maps.Clone(stateDiff.DeclaredV1Classes),
		ReplacedClasses:   maps.Clone(stateDiff.ReplacedClasses),
	}
}

func cloneStorageDiffs(storageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt) map[felt.Felt]map[felt.Felt]*felt.Felt {
	result := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for key := range storageDiffs {
		result[key] = maps.Clone(storageDiffs[key])
	}
	return result
}
