package starknetdata

import (
	"context"
	"errors"
	"strconv"

	"github.com/NethermindEth/juno/adapters/feeder2core"
	client "github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// StarknetData defines the function which are required to retrieve Starknet's state
//
//go:generate mockgen -destination=../mocks/mock_starknetdata.go -package=mocks github.com/NethermindEth/juno/starknetdata StarknetDataInterface
type StarknetDataInterface interface {
	BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error)
	BlockLatest(ctx context.Context) (*core.Block, error)
	BlockPending(ctx context.Context) (*core.Block, error)
	Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error)
	Class(ctx context.Context, classHash *felt.Felt) (core.Class, error)
	StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error)
	StateUpdatePending(ctx context.Context) (*core.StateUpdate, error)
}

type StarknetData struct {
	client client.FeederInterface
}

var _ StarknetDataInterface = &StarknetData{}

func NewStarknetData(cli *client.Client) *StarknetData {
	return &StarknetData{client.NewFeeder(cli)}
}

// BlockByNumber gets the block for a given block number from the feeder,
// then adapts it to the core.Block type.
func (s *StarknetData) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return s.block(ctx, strconv.FormatUint(blockNumber, 10))
}

// BlockLatest gets the latest block from the feeder,
// then adapts it to the core.Block type.
func (s *StarknetData) BlockLatest(ctx context.Context) (*core.Block, error) {
	return s.block(ctx, "latest")
}

// BlockPending gets the pending block from the feeder,
// then adapts it to the core.Block type.
func (s *StarknetData) BlockPending(ctx context.Context) (*core.Block, error) {
	return s.block(ctx, "pending")
}

func (s *StarknetData) block(ctx context.Context, blockID string) (*core.Block, error) {
	response, err := s.client.Block(ctx, blockID)
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
func (s *StarknetData) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := s.client.Transaction(ctx, transactionHash)
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
func (s *StarknetData) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	response, err := s.client.ClassDefinition(ctx, classHash)
	if err != nil {
		return nil, err
	}

	switch {
	case response.V1 != nil:
		compiledClass, cErr := s.client.CompiledClassDefinition(ctx, classHash)
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

func (s *StarknetData) stateUpdate(ctx context.Context, blockID string) (*core.StateUpdate, error) {
	response, err := s.client.StateUpdate(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return feeder2core.AdaptStateUpdate(response)
}

// StateUpdate gets the state update for a given block number from the feeder,
// then adapts it to the core.StateUpdate type.
func (s *StarknetData) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return s.stateUpdate(ctx, strconv.FormatUint(blockNumber, 10))
}

// StateUpdatePending gets the state update for the pending block from the feeder,
// then adapts it to the core.StateUpdate type.
func (s *StarknetData) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	return s.stateUpdate(ctx, "pending")
}
