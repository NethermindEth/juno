package statebackend

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
)

type deprecatedStateBackend struct {
	baseState
}

func (b *deprecatedStateBackend) StateCommitment() (felt.Felt, error) {
	//nolint:staticcheck,nolintlint // used by old state
	txn := b.database.NewIndexedBatch()
	height, err := core.GetChainHeight(txn)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return felt.Felt{}, nil
		}
		return felt.Felt{}, err
	}
	header, err := core.GetBlockHeaderByNumber(txn, height)
	if err != nil {
		return felt.Felt{}, err
	}
	return deprecatedstate.New(txn).Commitment(header.ProtocolVersion)
}

func (b *deprecatedStateBackend) HeadState() (core.StateReader, StateCloser, error) {
	//nolint:staticcheck,nolintlint // used by old state
	txn := b.database.NewIndexedBatch()

	height, err := core.GetChainHeight(txn)
	if err != nil {
		return nil, nil, err
	}

	// Note(Ege): Why do we fetch header here?
	_, err = core.GetBlockHeaderByNumber(txn, height)
	if err != nil {
		return nil, nil, err
	}

	return deprecatedstate.New(txn), NoopStateCloser, nil
}

func (b *deprecatedStateBackend) StateAtBlockNumber(
	blockNumber uint64,
) (core.StateReader, StateCloser, error) {
	//nolint:staticcheck,nolintlint // used by old state
	txn := b.database.NewIndexedBatch()

	// Note(Ege): Why do we fetch header here? To validate block exists?
	_, err := core.GetBlockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, nil, err
	}

	return deprecatedstate.NewHistory(
		deprecatedstate.New(txn),
		blockNumber,
	), NoopStateCloser, nil
}

func (b *deprecatedStateBackend) StateAtBlockHash(
	blockHash *felt.Felt,
) (core.StateReader, StateCloser, error) {
	//nolint:staticcheck,nolintlint // used by old state
	if blockHash.IsZero() {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		return deprecatedstate.New(txn), NoopStateCloser, nil
	}

	txn := b.database.NewIndexedBatch() //nolint:staticcheck // indexedBatch used by old state
	header, err := core.GetBlockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, nil, err
	}

	return deprecatedstate.NewHistory(
		deprecatedstate.New(txn),
		header.Number,
	), NoopStateCloser, nil
}

func (b *deprecatedStateBackend) Store(
	block *core.Block,
	blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	//nolint:staticcheck,nolintlint // used by old state
	err := b.database.Update(func(txn db.IndexedBatch) error {
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

		return writeBlockContent(
			b.database,
			txn,
			block,
			stateUpdate,
			blockCommitments,
			newClasses,
		)
	})
	if err != nil {
		return err
	}

	return b.runningFilter.Insert(
		block.EventsBloom,
		block.Number,
	)
}

func (b *deprecatedStateBackend) RevertHead() error {
	//nolint:staticcheck,nolintlint // used by old state
	err := b.database.Update(func(txn db.IndexedBatch) error {
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

		return deleteBlockContent(txn, txn, stateUpdate, blockNumber)
	})
	if err != nil {
		return err
	}
	return b.runningFilter.OnReorg()
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
	sign utils.BlockSignFunc,
) (SimulateResult, error) {
	//nolint:staticcheck,nolintlint // used by old state
	txn := b.database.NewIndexedBatch()
	defer txn.Close()

	err := updateStateRoots(deprecatedstate.New(txn), block, stateUpdate, newClasses)
	if err != nil {
		return SimulateResult{}, err
	}

	commitments, err := updateBlockHash(block, stateUpdate, b.network)
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
	sign utils.BlockSignFunc,
) error {
	//nolint:staticcheck,nolintlint // used by old state
	err := b.database.Update(func(txn db.IndexedBatch) error {
		err := updateStateRoots(deprecatedstate.New(txn), block, stateUpdate, newClasses)
		if err != nil {
			return err
		}

		commitments, err := updateBlockHash(block, stateUpdate, b.network)
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

		return writeBlockContent(
			txn,
			txn,
			block,
			stateUpdate,
			commitments,
			newClasses,
		)
	})
	if err != nil {
		return err
	}

	return b.runningFilter.Insert(block.EventsBloom, block.Number)
}
