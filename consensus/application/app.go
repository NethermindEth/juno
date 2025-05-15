package application

import (
	"context"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/mempool"
)

// Todo: merge the Application and Builder interfaces
type application[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	builder *builder.Builder
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](builder *builder.Builder) application[V, H, A] {
	return application[V, H, A]{
		builder: builder,
	}
}

// ExecuteTxns executes the provided transactions, and stores the result in the pending state
func (a *application[V, H, A]) ExecuteTxns(txns []mempool.BroadcastedTransaction) error {
	return a.builder.ExecuteTxns(txns)
}

// Commit writes the block and precommits to the db if the checks pass
func (a *application[V, H, A]) Commit(Height types.Height, block V, precommits []types.Precommit[H, A]) error {
	// Height check
	curHeight, err := a.Height()
	if err != nil {
		return err
	}
	if Height != curHeight+1 {
		return fmt.Errorf("fatal error: trying to Commit a block at the wrong height")
	}

	// Todo: Create signer
	// Todo: Store precommits
	return a.builder.Finalise(nil, true)
}

// Value executes a set of transactions from the mempool, and returns the resulting block
func (a *application[V, H, A]) Value() (V, error) {
	err := a.builder.InitPendingBlock()
	if err != nil {
		return nil, err
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err = a.builder.DepletePool(ctx)
	if err != nil {
		return nil, err
	}
	pending, err := a.builder.Pending()
	if err != nil {
		return nil, err
	}

	return pending, nil
}

// Valid rexecutes the transactions in the block, and runs all the checks required
// to store the block
func (a *application[V, H, A]) Valid(V) error {
	if err := a.builder.ClearPending(); err != nil {
		return err
	}

	// Todo: Execute txns in batch

	return a.builder.Finalise(nil, false)
}

// Height returns the latest commited height
func (a *application[V, H, A]) Height() (types.Height, error) {
	height, err := a.builder.Height()
	if err != nil {
		return types.Height(0), err
	}
	return types.Height(height), nil
}
