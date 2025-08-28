package builder

import (
	"maps"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type BuildState struct {
	Preconfirmed      *core.PreConfirmed
	L2GasConsumed     uint64
	RevealedBlockHash *felt.Felt
}

func (b *BuildState) PendingBlock() *core.Block {
	if b.Preconfirmed == nil {
		return nil
	}
	return b.Preconfirmed.Block
}

func (b *BuildState) ClearPending() error {
	b.L2GasConsumed = 0
	b.Preconfirmed = &core.PreConfirmed{}
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
		Preconfirmed:      clonePreconfirmed(b.Preconfirmed),
		RevealedBlockHash: b.RevealedBlockHash, // Safe to reuse an immutable value
		L2GasConsumed:     b.L2GasConsumed,     // Value, safe to shallow copy
	}
}

func clonePreconfirmed(preconfirmed *core.PreConfirmed) *core.PreConfirmed {
	return &core.PreConfirmed{
		Block:                 cloneBlock(preconfirmed.Block),
		StateUpdate:           cloneStateUpdate(preconfirmed.StateUpdate),
		NewClasses:            maps.Clone(preconfirmed.NewClasses),
		TransactionStateDiffs: preconfirmed.TransactionStateDiffs,
		CandidateTxs:          preconfirmed.CandidateTxs,
	}
}

func cloneBlock(block *core.Block) *core.Block {
	return &core.Block{
		Header:       utils.HeapPtr(*block.Header),
		Transactions: block.Transactions,
		Receipts:     block.Receipts,
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

func cloneStorageDiffs(
	storageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt,
) map[felt.Felt]map[felt.Felt]*felt.Felt {
	result := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for key := range storageDiffs {
		result[key] = maps.Clone(storageDiffs[key])
	}
	return result
}
