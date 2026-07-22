package l1

import (
	"context"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
)

// StateUpdate is the decoded form of the Starknet core contract's
// LogStateUpdate event in juno's own types
type StateUpdate struct {
	// L2BlockNumber is the Starknet block number being committed.
	L2BlockNumber uint64
	// L2BlockHash is the Starknet block hash for that block.
	L2BlockHash felt.Felt
	// StateRoot is the Starknet global state root after the block
	// (the "globalRoot" field in the on-chain event).
	StateRoot felt.Felt
	// L1RefHeight is the L1 block number where the commit was observed.
	// Used by the L1 sync loop to gate writes on L1 finality.
	L1RefHeight uint64
	// Removed is set when L1 signals that the log was rolled back by a
	// reorg.
	Removed bool
}

// L1StateProvider is how l1.Client follows the Ethereum L1 chain Starknet
// settles to: heights, LogStateUpdate events, and chain id.
//
//go:generate mockgen -destination=../mocks/mock_l1_state_provider.go -package=mocks github.com/NethermindEth/juno/l1 L1StateProvider
type L1StateProvider interface {
	ChainID(ctx context.Context) (*big.Int, error)
	FinalisedHeight(ctx context.Context) (uint64, error)
	LatestHeight(ctx context.Context) (uint64, error)
	WatchStateUpdate(ctx context.Context, updatesCh chan<- *StateUpdate) (Subscription, error)
	FilterStateUpdate(ctx context.Context, from, to uint64) ([]*StateUpdate, error)
	Close()
}
