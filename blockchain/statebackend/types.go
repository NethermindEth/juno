package statebackend

import (
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
)

// StateBackend is the interface for state operations in blockchain.
type StateBackend interface {
	HeadState() (core.StateReader, StateCloser, error)
	StateAtBlockNumber(blockNumber uint64) (core.StateReader, StateCloser, error)
	StateAtBlockHash(blockHash *felt.Felt) (core.StateReader, StateCloser, error)
	Store(
		block *core.Block,
		commitments *core.BlockCommitments,
		stateUpdate *core.StateUpdate,
		newClasses map[felt.Felt]core.ClassDefinition,
	) error
	RevertHead() error
	GetReverseStateDiff() (core.StateDiff, error)
	Simulate(
		block *core.Block,
		stateUpdate *core.StateUpdate,
		newClasses map[felt.Felt]core.ClassDefinition,
		sign core.BlockSignFunc,
	) (SimulateResult, error)
	Finalise(
		block *core.Block,
		stateUpdate *core.StateUpdate,
		newClasses map[felt.Felt]core.ClassDefinition,
		sign core.BlockSignFunc,
	) error
	VerifyBlockHash(
		b *core.Block,
		stateDiff *core.StateDiff,
	) (*core.BlockCommitments, error)
}

// StateCloser is called to release resources associated with a state reader.
type StateCloser = func() error

// NoopStateCloser is a StateCloser that does nothing.
var NoopStateCloser StateCloser = func() error { return nil }

type SimulateResult struct {
	BlockCommitments *core.BlockCommitments
	ConcatCount      felt.Felt
}

type baseState struct {
	database      db.KeyValueStore
	runningFilter *core.RunningEventFilter
	network       *networks.Network
}

func New(
	database db.KeyValueStore,
	runningFilter *core.RunningEventFilter,
	network *networks.Network,
	stateVersion bool,
) StateBackend {
	base := baseState{
		database:      database,
		runningFilter: runningFilter,
		network:       network,
	}

	if stateVersion {
		trieDB := triedb.New(database, nil)
		return &stateBackend{
			baseState: base,
			stateDB:   state.NewStateDB(database, trieDB),
		}
	}

	return &deprecatedStateBackend{baseState: base}
}
