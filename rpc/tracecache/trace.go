package tracecache

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/vm"
)

var ErrTargetNotFound = errors.New("transaction trace target not found")

type Source uint8

const (
	LocalVM Source = iota
	Feeder
)

const DefaultBlockCapacity = 256

// TransactionTrace holds a vm.TransactionTrace or starknet.TransactionTrace
// with its RPC adaptation metadata.
type TransactionTrace struct {
	Hash felt.Felt
	Type vm.TransactionType // Set by [FromFeeder]; VM traces carry their own type.
	Gas  core.GasConsumed

	vmTrace     *vm.TransactionTrace
	feederTrace *starknet.TransactionTrace
}

func (t *TransactionTrace) VMTrace() *vm.TransactionTrace {
	return t.vmTrace
}

func (t *TransactionTrace) FeederTrace() *starknet.TransactionTrace {
	return t.feederTrace
}

// BlockTrace holds version-neutral traces in block order; published data is read-only.
// InitialReads is nil when uncollected or unavailable ([Feeder]).
type BlockTrace struct {
	Complete     bool
	Source       Source
	Traces       []TransactionTrace
	InitialReads *vm.InitialReads
}

// Covers checks complete-block and requested InitialReads coverage.
// Feeder traces satisfy coverage even though they cannot provide InitialReads.
func (b *BlockTrace) Covers(initialReads bool) bool {
	return b.CoversTarget(nil, initialReads)
}

// CoversTarget is a cache-acceptance predicate: it reports whether the cached result
// supplies the requested coverage.
//
// A nil target requires a complete block.
// Feeder traces are accepted even when InitialReads are unavailable.
//
// Before serving a target, use ValidateTarget separately to verify its identity and bounds.
func (b *BlockTrace) CoversTarget(target *TransactionTarget, initialReads bool) bool {
	if !b.Complete && (target == nil || target.Index >= uint64(len(b.Traces))) {
		return false
	}
	return !initialReads || b.InitialReads != nil || b.Source == Feeder
}

// ValidateTarget checks that the target index identifies a trace with the requested hash.
func (b *BlockTrace) ValidateTarget(target *TransactionTarget) error {
	if target != nil && (target.Hash == nil || target.Index >= uint64(len(b.Traces)) ||
		!b.Traces[target.Index].Hash.Equal(target.Hash)) {
		return ErrTargetNotFound
	}
	return nil
}

// TransactionTarget pins a finalized transaction's identity and index. Nil requests a full block.
type TransactionTarget struct {
	Index uint64
	Hash  *felt.Felt
}
