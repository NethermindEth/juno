package sync

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
)

type CommittedBlock struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	NewClasses  map[felt.Felt]core.ClassDefinition
	Persisted   chan struct{} // This is used to signal that the block has been persisted
}

type DataSource interface {
	BlockByNumber(ctx context.Context, blockNumber uint64) (CommittedBlock, error)
	BlockHeaderLatest(ctx context.Context) (*core.Header, error)
	BlockPreLatest(ctx context.Context) (pending.PreLatest, error)
	PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (pending.PreConfirmed, error)
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
		Persisted:   make(chan struct{}),
	}, nil
}

func (f *feederGatewayDataSource) BlockHeaderLatest(ctx context.Context) (*core.Header, error) {
	header, err := f.starknetData.BlockHeaderLatest(ctx)
	if err != nil {
		return nil, err
	}
	return &header, nil
}

func (f *feederGatewayDataSource) BlockPreLatest(ctx context.Context) (pending.PreLatest, error) {
	pendingStateUpdate, pendingBlock, err := f.starknetData.StateUpdatePendingWithBlock(ctx)
	if err != nil {
		return pending.PreLatest{}, err
	}

	newClasses, err := f.fetchUnknownClasses(ctx, pendingStateUpdate)
	if err != nil {
		return pending.PreLatest{}, err
	}

	return pending.PreLatest{
		Block:       pendingBlock,
		StateUpdate: pendingStateUpdate,
		NewClasses:  newClasses,
	}, nil
}

func (f *feederGatewayDataSource) fetchUnknownClasses(
	ctx context.Context,
	stateUpdate *core.StateUpdate,
) (map[felt.Felt]core.ClassDefinition, error) {
	state, closer, err := f.blockchain.HeadState()
	if err != nil {
		// if err is db.ErrKeyNotFound we are on an empty DB
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		closer = func() error {
			return nil
		}
	}

	newClasses := make(map[felt.Felt]core.ClassDefinition)
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
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}
	for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}
	for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		if err = fetchIfNotFound(&classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}

	return newClasses, closer()
}

func (f *feederGatewayDataSource) PreConfirmedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (pending.PreConfirmed, error) {
	preConfirmed, err := f.starknetData.PreConfirmedBlockByNumber(ctx, blockNumber)
	if err != nil {
		return pending.PreConfirmed{}, err
	}

	return preConfirmed, nil
}
