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
// # Lifecycle
//
// A handler uses it when Cache.Acquire grants a lease for an unsatisfied request:
//
//  1. Plan the work with PlanRange for the requested transaction or block.
//
//  2. Prepare the starting state with ResumeState.
//
//  3. Execute transactions[Start:End] against that state. Keep both bounds unchanged.
//     Use OffsetExecutionError with Start when reporting VM errors to the RPC caller.
//
//  4. Package the suffix with FromVM and pass it to Combine to obtain a cacheable
//     trace prefix. It is complete when it covers every transaction in the block.
//
//  5. Call Lease.Publish on success.
//     Defer Lease.Abort to release it on failure.
//
// # Example
//
// A six-transaction block has two traces cached. A request arrives for transaction 4:
//   - Work needed: transactions [2, 5).
//   - Starting state: if the cached transactions changed a nonce from 6 to 7 to 8,
//     the VM starts with nonce 8.
//   - Result: the cache now covers transactions 0 through 4. A later request can
//     extend it to include transaction 5.
//
// # Ownership
//
// Keep borrowed state and class definitions usable through execution.
//
// Cached and combined traces share referenced data.
// That data must remain unchanged while in use.
type Range struct {
	Start, End uint64
	prefix     []TransactionTrace
	total      uint64
}

// PlanRange plans execution through a [TransactionTarget], or the block's end for nil.
//
// Supply the full ordered block as transactions. If cached is provided, it must
// be a LocalVM prefix of that block with non-nil vm.TransactionTrace.StateDiff
// fields (as produced by Combine).
// Prefix identity and diffs are the caller's responsibility.
//
// Collecting InitialReads requires full replay to observe reads from every transaction.
// Set target to nil and request InitialReads from the VM and FromVM as well.
func PlanRange(
	cached *BlockTrace,
	transactions []core.Transaction,
	target *TransactionTarget,
	initialReads bool,
) (*Range, error) {
	if target != nil && (target.Hash == nil || target.Index >= uint64(len(transactions)) ||
		!transactions[target.Index].Hash().Equal(target.Hash)) {
		return nil, ErrTargetNotFound
	}
	if target != nil && initialReads {
		return nil, errors.New("initial reads require a full block trace")
	}
	plan := &Range{End: uint64(len(transactions)), total: uint64(len(transactions))}
	if target != nil {
		plan.End = target.Index + 1
	}
	if cached != nil && !initialReads {
		if cached.Source != LocalVM {
			return nil, errors.New("cannot extend a feeder trace")
		}
		plan.prefix = cached.Traces
		plan.Start = uint64(len(plan.prefix))
	}
	if plan.Start > plan.End {
		return nil, fmt.Errorf(
			"cached trace prefix [0, %d) exceeds requested range [0, %d)",
			plan.Start,
			plan.End,
		)
	}
	return plan, nil
}

// ResumeState provides the VM's starting state for this range in blockNumber.
//
// Both core.StateReader arguments are borrowed:
//   - parent must read the state before the block.
//   - classes must resolve classes declared in the cached prefix.
//     These classes may not exist in the parent yet.
//
// With no prefix, execution starts directly from parent.
func (r *Range) ResumeState(
	parent, classes core.StateReader,
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

// Combine joins the executed suffix with the cached prefix into a BlockTrace
// ready for publication.
//
// executed must contain VM traces for exactly transactions[Start:End],
// in block order, with non-nil vm.TransactionTrace.StateDiff fields.
//
// The result is complete only if this range reaches the block's end.
// Referenced trace data remains shared and must not be mutated while in use.
func (r *Range) Combine(executed *BlockTrace) (*BlockTrace, error) {
	if executed.Source != LocalVM {
		return nil, errors.New("cannot combine non-VM traces")
	}
	if uint64(len(executed.Traces)) != r.End-r.Start {
		return nil, fmt.Errorf(
			"VM returned an unexpected trace range: expected [%d, %d) (%d traces), received %d traces",
			r.Start,
			r.End,
			r.End-r.Start,
			len(executed.Traces),
		)
	}
	for index := range executed.Traces {
		if executed.Traces[index].vmTrace == nil || executed.Traces[index].vmTrace.StateDiff == nil {
			return nil, fmt.Errorf("VM omitted state diff for transaction trace %d", r.Start+uint64(index))
		}
	}
	result := *executed
	result.Traces = make([]TransactionTrace, len(r.prefix)+len(executed.Traces))
	copy(result.Traces, r.prefix)
	copy(result.Traces[len(r.prefix):], executed.Traces)
	result.Complete = r.End == r.total
	return &result, nil
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
// Combine ensures newly appended traces have non-nil vm.TransactionTrace.StateDiff fields.
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
