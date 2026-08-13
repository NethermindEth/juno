package statebackend

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/pruner"
)

type deprecatedStateBackend struct {
	baseState
}

// readOnlyTxn adapts a KeyValueReader into the db.IndexedBatch shape that
// [deprecatedstate.New] requires. Read-only state views (HeadState,
// StateAtBlockNumber, StateAtBlockHash) never write, and an IndexedBatch read
// is not a snapshot either. Reading the database directly skips the per-call
// batch allocation and the overlay merge that every Get would otherwise pay.
type readOnlyTxn struct {
	db.KeyValueReader
}

var errReadOnlyStateView = errors.New("cannot write through a read-only state view")

func (readOnlyTxn) Put(_, _ []byte) error         { return errReadOnlyStateView }
func (readOnlyTxn) Delete(_ []byte) error         { return errReadOnlyStateView }
func (readOnlyTxn) DeleteRange(_, _ []byte) error { return errReadOnlyStateView }
func (readOnlyTxn) Write() error                  { return errReadOnlyStateView }
func (readOnlyTxn) Size() int                     { return 0 }
func (readOnlyTxn) Close() error                  { return nil }

func (b *deprecatedStateBackend) HeadState() (core.StateReader, StateCloser, error) {
	// Fail early if no block has been committed (no head state to open)
	// only the key's existence matters, not the height itself.
	if _, err := core.GetChainHeight(b.database); err != nil {
		return nil, nil, err
	}

	return deprecatedstate.New(readOnlyTxn{b.database}), NoopStateCloser, nil
}

func (b *deprecatedStateBackend) StateAtBlockNumber(
	blockNumber uint64,
) (core.StateReader, StateCloser, error) {
	err := pruner.RequireStateRetainedByBlockNumber(b.database, b.retentionFloor, blockNumber)
	if err != nil {
		return nil, nil, err
	}
	return deprecatedstate.NewHistory(
		deprecatedstate.New(readOnlyTxn{b.database}),
		blockNumber,
	), NoopStateCloser, nil
}

func (b *deprecatedStateBackend) StateAtBlockHash(
	blockHash *felt.Felt,
) (core.StateReader, StateCloser, error) {
	if blockHash.IsZero() {
		return deprecatedstate.New(readOnlyTxn{memory.New()}), NoopStateCloser, nil
	}

	blockNumber, err := pruner.BlockNumberByHashIfStateRetained(b.database, blockHash)
	if err != nil {
		return nil, nil, err
	}

	return deprecatedstate.NewHistory(
		deprecatedstate.New(readOnlyTxn{b.database}),
		blockNumber,
	), NoopStateCloser, nil
}

func (b *deprecatedStateBackend) Store(
	block *core.Block,
	blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	//nolint:staticcheck,nolintlint // used by old state
	return b.database.Update(func(txn db.IndexedBatch) error {
		if err := verifyBlockSuccession(txn, block); err != nil {
			return err
		}

		err := deprecatedstate.New(txn).Update(
			block.Header,
			stateUpdate,
			newClasses,
			false,
		)
		if err != nil {
			return err
		}

		err = writeBlockContent(
			b.database,
			txn,
			block,
			stateUpdate,
			blockCommitments,
			newClasses,
		)
		if err != nil {
			return err
		}

		return b.runningFilter.InsertWithBatch(txn, block.EventsBloom, block.Number)
	})
}

func (b *deprecatedStateBackend) RevertHead() error {
	//nolint:staticcheck,nolintlint // used by old state
	return b.database.Update(func(txn db.IndexedBatch) error {
		blockNumber, err := core.GetChainHeight(txn)
		if err != nil {
			return err
		}

		stateUpdate, err := core.GetStateUpdateByBlockNum(txn, blockNumber)
		if err != nil {
			return err
		}

		header, err := core.GetBlockHeaderByNumber(txn, blockNumber)
		if err != nil {
			return err
		}

		if err = deprecatedstate.New(txn).Revert(header, stateUpdate); err != nil {
			return err
		}

		if err := deleteBlockContent(txn, txn, stateUpdate, blockNumber); err != nil {
			return err
		}

		return b.runningFilter.OnReorgWithBatch(txn)
	})
}

func (b *deprecatedStateBackend) GetReverseStateDiff() (core.StateDiff, error) {
	//nolint:staticcheck,nolintlint // used by old state
	txn := b.database.NewIndexedBatch()
	blockNum, err := core.GetChainHeight(txn)
	if err != nil {
		return core.StateDiff{}, err
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(txn, blockNum)
	if err != nil {
		return core.StateDiff{}, err
	}

	reverseDiff, err := deprecatedstate.
		New(txn).
		GetReverseStateDiff(blockNum, stateUpdate.StateDiff)
	if err != nil {
		return core.StateDiff{}, err
	}
	return reverseDiff, nil
}

func (b *deprecatedStateBackend) Simulate(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign core.BlockSignFunc,
) (SimulateResult, error) {
	//nolint:staticcheck,nolintlint // used by old state
	txn := b.database.NewIndexedBatch()
	defer txn.Close()

	err := updateStateRoots(deprecatedstate.New(txn), block, stateUpdate, newClasses)
	if err != nil {
		return SimulateResult{}, err
	}

	commitments, err := updateBlockHash(block, stateUpdate, b.network, core.DeprecatedTrieBackend)
	if err != nil {
		return SimulateResult{}, err
	}

	concatCount := core.ConcatCounts(
		block.TransactionCount,
		block.EventCount,
		stateUpdate.StateDiff.Length(),
		block.L1DAMode,
	)

	// Note(Ege): Simulate is only called with nil `sign`, maybe we should remove it.
	if sign != nil {
		if err := signBlock(block, stateUpdate, sign); err != nil {
			return SimulateResult{}, err
		}
	}

	return SimulateResult{
		BlockCommitments: commitments,
		ConcatCount:      concatCount,
	}, nil
}

func (b *deprecatedStateBackend) Finalise(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign core.BlockSignFunc,
) error {
	//nolint:staticcheck,nolintlint // used by old state
	return b.database.Update(func(txn db.IndexedBatch) error {
		err := updateStateRoots(deprecatedstate.New(txn), block, stateUpdate, newClasses)
		if err != nil {
			return err
		}

		commitments, err := updateBlockHash(block, stateUpdate, b.network, core.DeprecatedTrieBackend)
		if err != nil {
			return err
		}

		// Note(Ege): Finalise is called with nil `sign` in `StoreGenesis` and in tests.
		// Maybe provide `StoreGenesis` its own path and make `sign` mandatory in this function.
		if sign != nil {
			if err := signBlock(block, stateUpdate, sign); err != nil {
				return err
			}
		}

		err = writeBlockContent(
			txn,
			txn,
			block,
			stateUpdate,
			commitments,
			newClasses,
		)
		if err != nil {
			return err
		}

		return b.runningFilter.InsertWithBatch(txn, block.EventsBloom, block.Number)
	})
}

func (b *deprecatedStateBackend) VerifyBlockHash(
	block *core.Block,
	stateDiff *core.StateDiff,
) (*core.BlockCommitments, error) {
	return core.VerifyBlockHash(block, b.network, stateDiff, core.DeprecatedTrieBackend)
}
