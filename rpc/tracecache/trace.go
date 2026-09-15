package tracecache

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/vm"
)

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
	Source       Source
	Traces       []TransactionTrace
	InitialReads *vm.InitialReads
}

// Covers checks requested read coverage, allowing unavailable feeder reads.
func (b *BlockTrace) Covers(initialReads bool) bool {
	return !initialReads || b.InitialReads != nil || b.Source == Feeder
}
