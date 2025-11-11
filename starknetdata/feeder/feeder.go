package feeder

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknetdata"
)

var _ starknetdata.StarknetData = (*Feeder)(nil)

const (
	latestID  = "latest"
	pendingID = "pending"
)

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
	return f.block(ctx, latestID)
}

// BlockPending gets the pending block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockPending(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, pendingID)
}

func (f *Feeder) block(ctx context.Context, blockID string) (*core.Block, error) {
	response, err := f.client.Block(ctx, blockID)
	if err != nil {
		return nil, err
	}

	if blockID == pendingID && response.Status != "PENDING" {
		return nil, errors.New("no pending block")
	}

	var sig *starknet.Signature
	if blockID != pendingID {
		sig, err = f.client.Signature(ctx, blockID)
		if err != nil {
			return nil, fmt.Errorf("get signature for block %q: %v", blockID, err)
		}
	}
	adaptedBlock, err := sn2core.AdaptBlock(response, sig)
	return &adaptedBlock, err
}

// Transaction gets the transaction for a given transaction hash from the feeder,
// then adapts it to the appropriate core.Transaction types.
func (f *Feeder) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := f.client.Transaction(ctx, transactionHash)
	if err != nil {
		return nil, err
	}
	tx, err := sn2core.AdaptTransaction(response.Transaction)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// Class gets the class for a given class hash from the feeder,
// then adapts it to the core.Class type.
func (f *Feeder) Class(ctx context.Context, classHash *felt.Felt) (core.ClassDefinition, error) {
	response, err := f.client.ClassDefinition(ctx, classHash)
	if err != nil {
		return nil, err
	}

	switch {
	case response.Sierra != nil:
		casmClass, cErr := f.client.CasmClassDefinition(ctx, classHash)
		if cErr != nil && !errors.Is(cErr, feeder.ErrDeprecatedCompiledClass) {
			return nil, cErr
		}

		return sn2core.AdaptSierraClass(response.Sierra, casmClass)
	case response.DeprecatedCairo != nil:
		return sn2core.AdaptDeprecatedCairoClass(response.DeprecatedCairo)
	default:
		return nil, errors.New("empty class")
	}
}

func (f *Feeder) stateUpdate(ctx context.Context, blockID string) (*core.StateUpdate, error) {
	response, err := f.client.StateUpdate(ctx, blockID)
	if err != nil {
		return nil, err
	}
	coreStateUpdate, err := sn2core.AdaptStateUpdate(response)
	return &coreStateUpdate, err
}

// StateUpdate gets the state update for a given block number from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, strconv.FormatUint(blockNumber, 10))
}

// StateUpdatePending gets the state update for the pending block from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, pendingID)
}

func (f *Feeder) stateUpdateWithBlock(ctx context.Context, blockID string) (*core.StateUpdate, *core.Block, error) {
	response, err := f.client.StateUpdateWithBlock(ctx, blockID)
	if err != nil {
		return nil, nil, err
	}

	if blockID == pendingID && response.Block.Status != "PENDING" {
		return nil, nil, errors.New("no pending block")
	}

	var sig *starknet.Signature
	if blockID != pendingID {
		sig, err = f.client.Signature(ctx, blockID)
		if err != nil {
			return nil, nil, err
		}
	}

	var adaptedState core.StateUpdate
	var adaptedBlock core.Block

	if adaptedState, err = sn2core.AdaptStateUpdate(response.StateUpdate); err != nil {
		return nil, nil, err
	}

	if adaptedBlock, err = sn2core.AdaptBlock(response.Block, sig); err != nil {
		return nil, nil, err
	}

	return &adaptedState, &adaptedBlock, nil
}

// StateUpdatePendingWithBlock gets both pending state update and pending block from the feeder,
// then adapts them to the core.StateUpdate and core.Block types respectively
func (f *Feeder) StateUpdatePendingWithBlock(ctx context.Context) (*core.StateUpdate, *core.Block, error) {
	return f.stateUpdateWithBlock(ctx, pendingID)
}

// StateUpdateWithBlock gets both state update and block for a given block number from the feeder,
// then adapts them to the core.StateUpdate and core.Block types respectively
func (f *Feeder) StateUpdateWithBlock(ctx context.Context, blockNumber uint64) (*core.StateUpdate, *core.Block, error) {
	return f.stateUpdateWithBlock(ctx, strconv.FormatUint(blockNumber, 10))
}

// PreConfirmedWithBlockByNumber gets both pending state update and pending block from the feeder,
// then adapts them to the core.PreConfirmed and list of transaction hashes types respectively
func (f *Feeder) PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (core.PreConfirmed, error) {
	response, err := f.client.PreConfirmedBlock(ctx, strconv.FormatUint(blockNumber, 10))
	if err != nil {
		return core.PreConfirmed{}, err
	}

	adaptedPreConfirmed, err := sn2core.AdaptPreConfirmedBlock(response, blockNumber)
	if err != nil {
		return core.PreConfirmed{}, err
	}

	return adaptedPreConfirmed, nil
}
