package p2p

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
)

type StartnetDataAdapter struct {
	base      starknetdata.StarknetData
	p2p       P2P
	network   utils.Network
	converter converter
}

func NewStarknetDataAdapter(base starknetdata.StarknetData, p2p P2P, blockchain *blockchain.Blockchain) starknetdata.StarknetData {
	return &StartnetDataAdapter{
		base:    base,
		p2p:     p2p,
		network: blockchain.Network(),
		converter: converter{
			blockchain: blockchain,
		},
	}
}

func (s *StartnetDataAdapter) BlockByNumber(ctx context.Context, blockNumber uint64) (block *core.Block, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	block, err = s.p2p.GetBlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch header via p2p")
	}

	return block, err
}

func (s *StartnetDataAdapter) BlockLatest(ctx context.Context) (*core.Block, error) {
	return s.base.BlockLatest(ctx)
}

func (s *StartnetDataAdapter) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	return s.base.Transaction(ctx, transactionHash)
}

func (s *StartnetDataAdapter) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	return s.base.Class(ctx, classHash)
}

func (s *StartnetDataAdapter) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return s.p2p.GetStateUpdate(ctx, blockNumber)
}

// Typecheck
var _ starknetdata.StarknetData = &StartnetDataAdapter{}
