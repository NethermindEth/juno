package tracecache

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/vm"
)

// Range describes the missing interval [Start, End). Callers retain state lifetime,
// transaction preparation, VM invocation, and error mapping.
type Range struct {
	Start, End uint64
	prefix     []TransactionTrace
	total      uint64
}

// PlanRange selects the missing suffix, or a full replay for initial reads.
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
		return nil, errors.New("cached trace prefix exceeds requested range")
	}
	return plan, nil
}

// ResumeState borrows the readers; the caller remains responsible for closing them.
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

func (r *Range) Combine(executed *BlockTrace) (*BlockTrace, error) {
	if executed.Source != LocalVM || uint64(len(executed.Traces)) != r.End-r.Start {
		return nil, errors.New("VM returned an unexpected trace range")
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

// OffsetExecutionError converts a suffix error index to a block index.
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

// checkpointFromTraces rebuilds the continuation checkpoint from cached
// per-transaction state diffs, avoiding a duplicate cumulative diff in the cache.
// Combine ensures newly appended traces have non-nil state diffs.
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
