package tracecache

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/vm"
)

// Range lets RPC handlers extend a cached BlockTrace without re-executing its
// transactions. Cached vm.StateDiff values provide the checkpoint the VM needs to continue.
// The handler owns execution, state-reader lifetime, and cache publication.
//
// Lifecycle:
// A handler uses it when [Cache.Acquire] grants a lease for an unsatisfied request:
//  1. Plan the work with [PlanRange] for the requested transaction or block.
//  2. Prepare the starting state with [Range.ResumeState].
//  3. Execute transactions[Start:End] against that state.
//     Use [OffsetExecutionError] with Start when reporting VM errors to the RPC caller.
//  4. Package the suffix with [FromVM] and call [BlockTrace.Combine] with the range.
//  5. Call [Lease.Publish] on success, or [Lease.Release] on failure.
//
// Example Workflow:
// A six-transaction block has two traces cached. A request arrives for transaction 4:
//   - Work needed: transactions [2, 5).
//   - Starting state: if the cached transactions changed a nonce from 6 to 7 to 8,
//     the VM starts with nonce 8.
//   - Result: the cache now covers transactions 0 through 4. A later request can
//     extend it to include transaction 5.
//
// See [Cache] for the mutability contract for published traces.
type Range struct {
	Start  uint64
	End    uint64
	prefix []TransactionTrace
	total  uint64
}

// PlanRange determines which block transactions need execution.
// The returned range uses an inclusive Start and exclusive End.
//
// Parameters:
//   - cached: the block trace returned alongside the lease by
//     [Cache.Acquire].
//   - transactions: the full ordered transaction list loaded for the block,
//     including transactions already covered by cached.
//   - target: the requested transaction's index and hash, assembled by the
//     RPC handler. Nil requests execution through the block's end.
//   - initialReads: when true, the entire block must be replayed from the beginning.
//     The target must be nil or the last transaction.
//     Request initial reads from the VM and pass true to [FromVM].
func PlanRange(
	cached *BlockTrace,
	transactions []core.Transaction,
	target *TransactionTarget,
	initialReads bool,
) (Range, error) {
	invalidTarget := target != nil && (target.Hash == nil ||
		target.Index >= uint64(len(transactions)) ||
		!transactions[target.Index].Hash().Equal((*felt.Felt)(target.Hash)))
	if invalidTarget {
		return Range{}, ErrTargetNotFound
	}
	if initialReads && target != nil && target.Index+1 != uint64(len(transactions)) {
		return Range{}, errors.New("initial reads require a full block trace")
	}
	plan := Range{End: uint64(len(transactions)), total: uint64(len(transactions))}
	if target != nil {
		plan.End = target.Index + 1
	}
	if cached != nil && !initialReads {
		if cached.Source != LocalVM {
			return Range{}, errors.New("cannot extend a feeder trace")
		}
		plan.prefix = cached.Traces
		plan.Start = uint64(len(plan.prefix))
	}
	if plan.Start > plan.End {
		return Range{}, fmt.Errorf(
			"cached trace prefix [0, %d) exceeds requested range [0, %d)",
			plan.Start,
			plan.End,
		)
	}
	return plan, nil
}

// ResumeState provides the VM's starting state for this range.
// Reconstructs the state after the cached prefix so that the VM can execute
// the remaining transactions without replaying that prefix.
//
// Parameters:
//   - parent: the state reader for the state immediately before the block.
//   - classes: the state reader used to load classes declared in the cached
//     prefix, typically the chain's head state.
//   - blockNumber: the number of the block being traced.
func (r *Range) ResumeState(
	parent core.StateReader,
	classes core.StateReader,
	blockNumber uint64,
) (core.StateReader, error) {
	if r.Start == 0 {
		return parent, nil
	}
	checkpoint := checkpointFromTraces(r.prefix)
	declared, err := loadCheckpointClasses(&checkpoint, classes)
	if err != nil {
		return nil, err
	}
	return pending.NewState(&checkpoint, declared, parent, blockNumber), nil
}

// Combine prepends the range's cached prefix to b's executed suffix and returns b.
//
// Parameters:
//   - r: the non-nil range used to execute transactions[r.Start:r.End].
//
// The receiver is marked complete only if the range reaches the block's end.
func (b *BlockTrace) Combine(r *Range) (*BlockTrace, error) {
	if b.Source != LocalVM {
		return nil, errors.New("cannot combine non-VM traces")
	}
	if uint64(len(b.Traces)) != r.End-r.Start {
		return nil, fmt.Errorf(
			"VM returned an unexpected trace range: expected [%d, %d) (%d traces), received %d traces",
			r.Start,
			r.End,
			r.End-r.Start,
			len(b.Traces),
		)
	}
	for index := range b.Traces {
		if b.Traces[index].vmTrace == nil || b.Traces[index].vmTrace.StateDiff == nil {
			return nil, fmt.Errorf("VM omitted state diff for transaction trace %d", r.Start+uint64(index))
		}
	}
	traces := make([]TransactionTrace, len(r.prefix)+len(b.Traces))
	copy(traces, r.prefix)
	copy(traces[len(r.prefix):], b.Traces)
	b.Traces = traces
	b.Complete = r.End == r.total
	return b, nil
}

// OffsetExecutionError converts a vm.TransactionExecutionError index from
// suffix-relative to block-relative.
func OffsetExecutionError(err error, offset uint64) error {
	if err == nil || offset == 0 {
		return err
	}
	var transactionErr vm.TransactionExecutionError
	if !errors.As(err, &transactionErr) {
		return err
	}
	transactionErr.Index += offset
	return transactionErr
}

// loadCheckpointClasses loads definitions for classes declared in the cached
// prefix so ResumeState can make them available to the remaining transactions.
func loadCheckpointClasses(
	diff *core.StateDiff,
	classLookup core.StateReader,
) (map[felt.Felt]core.ClassDefinition, error) {
	classes := make(
		map[felt.Felt]core.ClassDefinition,
		len(diff.DeclaredV0Classes)+len(diff.DeclaredV1Classes),
	)
	for _, hash := range diff.DeclaredV0Classes {
		classes[*hash] = nil
	}
	for hash := range diff.DeclaredV1Classes {
		classes[hash] = nil
	}
	for hash := range classes {
		declared, err := classLookup.Class(&hash)
		if err != nil {
			return nil, err
		}
		classes[hash] = declared.Class
	}
	return classes, nil
}

// checkpointFromTraces rebuilds a core.StateDiff checkpoint from cached
// vm.StateDiff values, avoiding a duplicate cumulative diff in the cache.
func checkpointFromTraces(traces []TransactionTrace) core.StateDiff {
	result := core.EmptyStateDiff()
	for index := range traces {
		mergeVMStateDiff(&result, traces[index].vmTrace.StateDiff)
	}
	return result
}

func mergeVMStateDiff(result *core.StateDiff, diff *vm.StateDiff) {
	for storageIndex := range diff.StorageDiffs {
		storage := &diff.StorageDiffs[storageIndex]
		entries, found := result.StorageDiffs[storage.Address]
		if !found {
			entries = make(map[felt.Felt]*felt.Felt, len(storage.StorageEntries))
			result.StorageDiffs[storage.Address] = entries
		}
		for entryIndex := range storage.StorageEntries {
			entry := &storage.StorageEntries[entryIndex]
			entries[entry.Key] = entry.Value.Clone()
		}
	}
	for nonceIndex := range diff.Nonces {
		nonce := &diff.Nonces[nonceIndex]
		result.Nonces[nonce.ContractAddress] = nonce.Nonce.Clone()
	}
	for deployedIndex := range diff.DeployedContracts {
		deployed := &diff.DeployedContracts[deployedIndex]
		result.DeployedContracts[deployed.Address] = deployed.ClassHash.Clone()
	}
	for _, hash := range diff.DeprecatedDeclaredClasses {
		result.DeclaredV0Classes = append(result.DeclaredV0Classes, hash.Clone())
	}
	for declaredIndex := range diff.DeclaredClasses {
		declared := &diff.DeclaredClasses[declaredIndex]
		result.DeclaredV1Classes[declared.ClassHash] = declared.CompiledClassHash.Clone()
	}
	for replacedIndex := range diff.ReplacedClasses {
		replaced := &diff.ReplacedClasses[replacedIndex]
		result.ReplacedClasses[replaced.ContractAddress] = replaced.ClassHash.Clone()
	}
	for migratedIndex := range diff.MigratedCompiledClasses {
		migrated := &diff.MigratedCompiledClasses[migratedIndex]
		result.MigratedClasses[migrated.ClassHash] = migrated.CompiledClassHash
	}
}
