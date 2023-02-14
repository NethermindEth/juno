package starknetdata

import (
	"context"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// StarknetData defines the function which are required to retrieve Starknet's state
type StarknetData interface {
	BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error)
	Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error)
	Class(ctx context.Context, classHash *felt.Felt) (*core.Class, error)
	StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error)
}
