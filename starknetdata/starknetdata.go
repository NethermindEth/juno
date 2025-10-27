package starknetdata

import (
	"context"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// StarknetData defines the function which are required to retrieve Starknet's state
//
//go:generate mockgen -destination=../mocks/mock_starknetdata.go -package=mocks github.com/NethermindEth/juno/starknetdata StarknetData
type StarknetData interface {
	BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error)
	BlockLatest(ctx context.Context) (*core.Block, error)
	BlockPending(ctx context.Context) (*core.Block, error)
	Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error)
	Class(ctx context.Context, classHash *felt.Felt) (core.ClassDefinition, error)
	StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error)
	StateUpdatePending(ctx context.Context) (*core.StateUpdate, error)
	StateUpdateWithBlock(ctx context.Context, blockNumber uint64) (*core.StateUpdate, *core.Block, error)
	StateUpdatePendingWithBlock(ctx context.Context) (*core.StateUpdate, *core.Block, error)
	PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (core.PreConfirmed, error)
}
