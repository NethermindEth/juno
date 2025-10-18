package datasource

import (
	"context"
	"errors"
	"fmt"
	syncmap "sync"
	"sync/atomic"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/proposer"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/sync"
)

const maxCommitHistory = 1024 // TODO: make this configurable

type consensusDataSource[V types.Hashable[H], H types.Hash] struct {
	commitListener driver.CommitListener[V, H]
	proposer       proposer.Proposer[V, H]
	cache          syncmap.Map
	latest         atomic.Uint64
}

func New[V types.Hashable[H], H types.Hash](
	commitListener driver.CommitListener[V, H],
	proposer proposer.Proposer[V, H],
) *consensusDataSource[V, H] {
	return &consensusDataSource[V, H]{
		commitListener: commitListener,
		proposer:       proposer,
		cache:          syncmap.Map{},
		latest:         atomic.Uint64{},
	}
}

func (c *consensusDataSource[V, H]) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case committedBlock, ok := <-c.commitListener.Listen():
			if !ok {
				return nil
			}
			blockNumber := committedBlock.Block.Number

			c.cache.Store(blockNumber, &committedBlock)
			c.latest.Store(blockNumber)
			c.cache.Delete(blockNumber - maxCommitHistory)
		}
	}
}

func (c *consensusDataSource[V, H]) BlockByNumber(ctx context.Context, blockNumber uint64) (sync.CommittedBlock, error) {
	committedBlock, ok := c.cache.Load(blockNumber)
	if !ok {
		return sync.CommittedBlock{}, errors.New("block not found in cache")
	}

	return *committedBlock.(*sync.CommittedBlock), nil
}

func (c *consensusDataSource[V, H]) BlockLatest(ctx context.Context) (*core.Block, error) {
	committedBlock, err := c.BlockByNumber(ctx, c.latest.Load())
	if err != nil {
		return nil, err
	}

	return committedBlock.Block, nil
}

func (c *consensusDataSource[V, H]) BlockPending(ctx context.Context) (core.Pending, error) {
	return core.Pending{}, errors.New("not implemented") // TODO: Revise this
}

func (c *consensusDataSource[V, H]) PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (core.PreConfirmed, error) {
	preconfirmed := c.proposer.Preconfirmed()
	if preconfirmed.Block.Number != blockNumber {
		return core.PreConfirmed{}, fmt.Errorf("block %d is not preconfirmed", blockNumber)
	}
	return *preconfirmed, nil
}
