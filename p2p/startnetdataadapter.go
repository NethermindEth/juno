package p2p

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
)

type StartnetDataAdapter struct {
	base starknetdata.StarknetData
	p2p  P2P
}

func NewStarknetDataAdapter(base starknetdata.StarknetData, p2p P2P) starknetdata.StarknetData {
	return &StartnetDataAdapter{
		base: base,
		p2p:  p2p,
	}
}

func (s *StartnetDataAdapter) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	fmt.Printf("Requesting for block %d\n", blockNumber)
	header, err := s.p2p.GetBlockHeaderByNumber(ctx, blockNumber)
	if err != nil {
		return nil, err
	}

	block, err := s.p2p.GetBlockByHash(ctx, header.Hash)
	if err != nil {
		return nil, err
	}

	gatewayBlock, err := s.base.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, err
	}

	if block != gatewayBlock {
		fmt.Printf("Block mismatch")
	}

	return gatewayBlock, err
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
	return s.base.StateUpdate(ctx, blockNumber)
}

// Typecheck
var _ starknetdata.StarknetData = &StartnetDataAdapter{}
