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

// TransactionTrace holds one source trace and its RPC adaptation metadata.
type TransactionTrace struct {
	Hash felt.Felt
	Type vm.TransactionType
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

// BlockTrace holds version-neutral traces in block order. Once published,
// all slices and referenced data are read-only, including for adapters.
// InitialReads is nil when uncollected or unavailable (Feeder).
type BlockTrace struct {
	Source       Source
	Traces       []TransactionTrace
	InitialReads *vm.InitialReads
}

// DefaultCapacity is the number of finalized blocks retained across RPC versions.
const DefaultCapacity = 256

// Covers checks block coverage and requested reads; adapters handle unavailable feeder reads.
func (b *BlockTrace) Covers(initialReads bool) bool {
	return !initialReads || b.InitialReads != nil || b.Source == Feeder
}
