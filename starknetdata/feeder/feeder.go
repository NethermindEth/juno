package feeder

import (
	"context"
	"errors"
	"strconv"

	"github.com/NethermindEth/juno/adapters/feeder2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
)

var _ starknetdata.StarknetData = (*Feeder)(nil)

type Feeder struct {
	client *feeder.Client
}

func New(client *feeder.Client) *Feeder {
	return &Feeder{
		client: client,
	}
}

// BlockByNumber gets the block for a given block number from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return f.block(ctx, strconv.FormatUint(blockNumber, 10))
}

// BlockLatest gets the latest block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockLatest(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, "latest")
}

// BlockPending gets the pending block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockPending(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, "pending")
}

func (f *Feeder) block(ctx context.Context, blockID string) (*core.Block, error) {
	response, err := f.client.Block(ctx, blockID)
	if err != nil {
		return nil, err
	}

	if blockID == "pending" && response.Status != "PENDING" {
		return nil, errors.New("no pending block")
	}
	return feeder2core.AdaptBlock(response)
}

// Transaction gets the transaction for a given transaction hash from the feeder,
// then adapts it to the appropriate core.Transaction types.
func (f *Feeder) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := f.client.Transaction(ctx, transactionHash)
	if err != nil {
		return nil, err
	}

	tx, err := feeder2core.AdaptTransaction(response.Transaction)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// Class gets the class for a given class hash from the feeder,
// then adapts it to the core.Class type.
func (f *Feeder) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	response, err := f.client.ClassDefinition(ctx, classHash)
	if err != nil {
		return nil, err
	}

	switch {
	case response.V1 != nil:
		compiledClass, cErr := f.client.CompiledClassDefinition(ctx, classHash)
		if cErr != nil {
			return nil, cErr
		}

		return feeder2core.AdaptCairo1Class(response.V1, compiledClass)
	case response.V0 != nil:
		return feeder2core.AdaptCairo0Class(response.V0)
	default:
		return nil, errors.New("empty class")
	}
}

func (f *Feeder) stateUpdate(ctx context.Context, blockID string) (*core.StateUpdate, error) {
	response, err := f.client.StateUpdate(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return feeder2core.AdaptStateUpdate(response)
}

// StateUpdate gets the state update for a given block number from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, strconv.FormatUint(blockNumber, 10))
}

// StateUpdatePending gets the state update for the pending block from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, "pending")
}

func (f *Feeder) stateUpdateWithBlock(ctx context.Context, blockID string) (*core.StateUpdate, *core.Block, error) {
	response, err := f.client.StateUpdateWithBlock(ctx, blockID)
	if err != nil {
		// TODO: remove once the new feeder is available
		if err.Error() == "400 Bad Request" {
			if blockID == "pending" {
				return f.stateUpdateWithBlockPendingFallback(ctx)
			}

			return f.stateUpdateWithBlockFallback(ctx, blockID)
		}
		return nil, nil, err
	}

	if blockID == "pending" && response.Block.Status != "PENDING" {
		return nil, nil, errors.New("no pending block")
	}

	var adaptedState *core.StateUpdate
	var adaptedBlock *core.Block

	if adaptedState, err = feeder2core.AdaptStateUpdate(response.StateUpdate); err != nil {
		return nil, nil, err
	}

	if adaptedBlock, err = feeder2core.AdaptBlock(response.Block); err != nil {
		return nil, nil, err
	}

	return adaptedState, adaptedBlock, nil
}

// StateUpdatePendingWithBlock gets both pending state update and pending block from the feeder,
// then adapts them to the core.StateUpdate and core.Block types respectively
func (f *Feeder) StateUpdatePendingWithBlock(ctx context.Context) (*core.StateUpdate, *core.Block, error) {
	return f.stateUpdateWithBlock(ctx, "pending")
}

// StateUpdateWithBlock gets both state update and block for a given block number from the feeder,
// then adapts them to the core.StateUpdate and core.Block types respectively
func (f *Feeder) StateUpdateWithBlock(ctx context.Context, blockNumber uint64) (*core.StateUpdate, *core.Block, error) {
	return f.stateUpdateWithBlock(ctx, strconv.FormatUint(blockNumber, 10))
}

// TODO: remove once new feeder endpoint is available
func (f *Feeder) stateUpdateWithBlockFallback(ctx context.Context, height string) (*core.StateUpdate, *core.Block, error) {
	block, err := f.block(ctx, height)
	if err != nil {
		return nil, nil, err
	}
	stateUpdate, err := f.stateUpdate(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	return stateUpdate, block, nil
}

// TODO: remove once new feeder endpoint hits mainnet
func (f *Feeder) stateUpdateWithBlockPendingFallback(ctx context.Context) (*core.StateUpdate, *core.Block, error) {
	pendingBlock, err := f.block(ctx, "pending")
	if err != nil {
		return nil, nil, err
	}

	pendingStateUpdate, err := f.stateUpdate(ctx, "pending")
	if err != nil {
		return nil, nil, err
	}

	return pendingStateUpdate, pendingBlock, nil
}
