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

// TransactionTrace holds one source trace and its RPC adaptation metadata.
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

// Covers checks requested read coverage, allowing unavailable feeder reads.
// It also requires complete block coverage.
func (b *BlockTrace) Covers(initialReads bool) bool {
	if !b.Complete {
		return false
	}
	return !initialReads || b.InitialReads != nil || b.Source == Feeder
}

// CoversTarget checks coverage through the requested transaction and requested initial reads.
// The target must be non-nil. Use [BlockTrace.Covers] for whole-block coverage.
func (b *BlockTrace) CoversTarget(target *TransactionTarget, initialReads bool) bool {
	if target.Index >= uint64(len(b.Traces)) && !b.Complete {
		return false
	}
	return !initialReads || b.InitialReads != nil || b.Source == Feeder
}

// ValidTarget reports whether the target index identifies a trace with the requested hash.
func (b *BlockTrace) ValidTarget(target *TransactionTarget) bool {
	return target == nil || (target.Hash != nil &&
		target.Index < uint64(len(b.Traces)) &&
		b.Traces[target.Index].Hash.Equal((*felt.Felt)(target.Hash)))
}

// TransactionTarget identifies the transaction to trace.
// A nil target requests a full block trace.
type TransactionTarget struct {
	Index uint64
	Hash  *felt.TransactionHash
}
