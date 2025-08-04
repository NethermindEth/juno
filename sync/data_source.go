package sync

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata"
)

type CommittedBlock struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	NewClasses  map[felt.Felt]core.Class
}

type DataSource interface {
	BlockByNumber(ctx context.Context, blockNumber uint64) (CommittedBlock, error)
	BlockLatest(ctx context.Context) (*core.Block, error)
	BlockPending(ctx context.Context) (Pending, error)
	PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (core.PreConfirmed, error)
}

type feederGatewayDataSource struct {
	blockchain   *blockchain.Blockchain
	starknetData starknetdata.StarknetData
}

func NewFeederGatewayDataSource(blockchain *blockchain.Blockchain, starknetData starknetdata.StarknetData) DataSource {
	return &feederGatewayDataSource{
		blockchain:   blockchain,
		starknetData: starknetData,
	}
}

func (f *feederGatewayDataSource) BlockByNumber(ctx context.Context, blockNumber uint64) (CommittedBlock, error) {
	stateUpdate, block, err := f.starknetData.StateUpdateWithBlock(ctx, blockNumber)
	if err != nil {
		return CommittedBlock{}, err
	}

	newClasses, err := f.fetchUnknownClasses(ctx, stateUpdate)
	if err != nil {
		return CommittedBlock{}, err
	}

	return CommittedBlock{
		Block:       block,
		StateUpdate: stateUpdate,
		NewClasses:  newClasses,
	}, nil
}

func (f *feederGatewayDataSource) BlockLatest(ctx context.Context) (*core.Block, error) {
	return f.starknetData.BlockLatest(ctx)
}

func (f *feederGatewayDataSource) BlockPending(ctx context.Context) (Pending, error) {
	pendingStateUpdate, pendingBlock, err := f.starknetData.StateUpdatePendingWithBlock(ctx)
	if err != nil {
		return Pending{}, err
	}

	newClasses, err := f.fetchUnknownClasses(ctx, pendingStateUpdate)
	if err != nil {
		return Pending{}, err
	}

	return Pending{
		Block:       pendingBlock,
		StateUpdate: pendingStateUpdate,
		NewClasses:  newClasses,
	}, nil
}

func (f *feederGatewayDataSource) fetchUnknownClasses(
	ctx context.Context,
	stateUpdate *core.StateUpdate,
) (map[felt.Felt]core.Class, error) {
	state, err := f.blockchain.HeadState()
	if err != nil {
		// if err is db.ErrKeyNotFound we are on an empty DB
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
	}

	newClasses := make(map[felt.Felt]core.Class)
	fetchIfNotFound := func(classHash *felt.Felt) error {
		if _, ok := newClasses[*classHash]; ok {
			return nil
		}

		stateErr := db.ErrKeyNotFound
		if state != nil {
			_, stateErr = state.Class(classHash)
		}

		if errors.Is(stateErr, db.ErrKeyNotFound) {
			class, fetchErr := f.starknetData.Class(ctx, classHash)
			if fetchErr == nil {
				newClasses[*classHash] = class
			}
			return fetchErr
		}
		return stateErr
	}

	for _, classHash := range stateUpdate.StateDiff.DeployedContracts {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, err
		}
	}
	for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, err
		}
	}
	for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		if err = fetchIfNotFound(&classHash); err != nil {
			return nil, err
		}
	}

	return newClasses, nil
}

func (f *feederGatewayDataSource) PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (core.PreConfirmed, error) {
	preConfirmed, err := f.starknetData.PreConfirmedBlockByNumber(ctx, blockNumber)
	if err != nil {
		return core.PreConfirmed{}, err
	}

	h, err := f.blockchain.HeadsHeader()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return core.PreConfirmed{}, err
	} else if err == nil {
		preConfirmed.StateUpdate.OldRoot = h.GlobalStateRoot
	}

	return preConfirmed, nil
}
