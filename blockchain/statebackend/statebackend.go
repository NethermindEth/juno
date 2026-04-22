package statebackend

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
)

type stateBackend struct {
	baseState
	stateDB *state.StateDB
}

func (b *stateBackend) HeadState() (core.StateReader, StateCloser, error) {
	height, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, nil, err
	}

	header, err := core.GetBlockHeaderByNumber(b.database, height)
	if err != nil {
		return nil, nil, err
	}

	st, err := state.NewStateReader(header.GlobalStateRoot, b.stateDB)
	if err != nil {
		return nil, nil, err
	}
	return st, NoopStateCloser, nil
}

func (b *stateBackend) StateAtBlockNumber(
	blockNumber uint64,
) (core.StateReader, StateCloser, error) {
	header, err := core.GetBlockHeaderByNumber(b.database, blockNumber)
	if err != nil {
		return nil, nil, err
	}

	history, err := state.NewStateHistory(blockNumber, header.GlobalStateRoot, b.stateDB)
	if err != nil {
		return nil, nil, err
	}
	return &history, NoopStateCloser, nil
}

func (b *stateBackend) StateAtBlockHash(
	blockHash *felt.Felt,
) (core.StateReader, StateCloser, error) {
	if blockHash.IsZero() {
		st, err := state.NewStateReader(&felt.Zero, b.stateDB)
		if err != nil {
			return nil, nil, err
		}
		return st, NoopStateCloser, nil
	}

	blockNumber, err := core.GetBlockHeaderNumberByHash(b.database, blockHash)
	if err != nil {
		return nil, nil, err
	}

	return b.StateAtBlockNumber(blockNumber)
}

func (b *stateBackend) Store(
	block *core.Block,
	blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	err := b.database.Write(func(batch db.Batch) error {
		if err := verifyBlockSuccession(b.database, block); err != nil {
			return err
		}

		st, err := state.New(stateUpdate.OldRoot, b.stateDB, batch)
		if err != nil {
			return err
		}
		if err := st.Update(block.Header, stateUpdate, newClasses, false); err != nil {
			return err
		}

		return writeBlockContent(
			b.database,
			batch,
			block,
			stateUpdate,
			blockCommitments,
			newClasses,
		)
	})
	if err != nil {
		return err
	}

	return b.runningFilter.Insert(block.EventsBloom, block.Number)
}

func (b *stateBackend) RevertHead() error {
	err := b.database.Write(func(batch db.Batch) error {
		blockNumber, err := core.GetChainHeight(b.database)
		if err != nil {
			return err
		}

		stateUpdate, err := core.GetStateUpdateByBlockNum(b.database, blockNumber)
		if err != nil {
			return err
		}

		header, err := core.GetBlockHeaderByNumber(b.database, blockNumber)
		if err != nil {
			return err
		}

		st, err := state.New(stateUpdate.NewRoot, b.stateDB, batch)
		if err != nil {
			return err
		}

		if err = st.Revert(header, stateUpdate); err != nil {
			return err
		}

		return deleteBlockContent(b.database, batch, stateUpdate, blockNumber)
	})
	if err != nil {
		return err
	}
	return b.runningFilter.OnReorg()
}

func (b *stateBackend) GetReverseStateDiff() (core.StateDiff, error) {
	blockNum, err := core.GetChainHeight(b.database)
	if err != nil {
		return core.StateDiff{}, err
	}
	stateUpdate, err := core.GetStateUpdateByBlockNum(b.database, blockNum)
	if err != nil {
		return core.StateDiff{}, err
	}
	st, err := state.NewStateReader(stateUpdate.NewRoot, b.stateDB)
	if err != nil {
		return core.StateDiff{}, err
	}

	return st.GetReverseStateDiff(blockNum, stateUpdate.StateDiff)
}

func (b *stateBackend) Simulate(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign core.BlockSignFunc,
) (SimulateResult, error) {
	batch := b.database.NewBatch()
	defer batch.Close()

	st, err := state.New(stateUpdate.OldRoot, b.stateDB, batch)
	if err != nil {
		return SimulateResult{}, err
	}
	if err := updateStateRoots(st, block, stateUpdate, newClasses); err != nil {
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

func (b *stateBackend) Finalise(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign core.BlockSignFunc,
) error {
	err := b.database.Write(func(batch db.Batch) error {
		st, err := state.New(stateUpdate.OldRoot, b.stateDB, batch)
		if err != nil {
			return err
		}
		if err := updateStateRoots(st, block, stateUpdate, newClasses); err != nil {
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
			b.database,
			batch,
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
