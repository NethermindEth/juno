package p2p

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/pkg/errors"
	"sync"
)

type StartnetDataAdapter struct {
	base      starknetdata.StarknetData
	p2p       BlockSyncPeerManager
	network   utils.Network
	converter converter

	lruMtx *sync.Mutex

	// The class is included in block tx. So we'll add them to an LRU to be used later to prevent re-fetching.
	classesLru *simplelru.LRU
}

func NewStarknetDataAdapter(base starknetdata.StarknetData, p2p BlockSyncPeerManager, blockchain *blockchain.Blockchain) starknetdata.StarknetData {
	lru, err := simplelru.NewLRU(16000, func(key interface{}, value interface{}) {})
	if err != nil {
		panic(err)
	}

	return &StartnetDataAdapter{
		base:    base,
		p2p:     p2p,
		network: blockchain.Network(),
		converter: converter{
			classprovider: &blockchainClassProvider{
				blockchain: blockchain,
			},
		},

		lruMtx:     &sync.Mutex{},
		classesLru: lru,
	}
}

func (s *StartnetDataAdapter) BlockByNumber(ctx context.Context, blockNumber uint64) (block *core.Block, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in get block by number", r)
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	block, declaredClasses, err := s.p2p.GetBlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch header via p2p")
	}

	for key, class := range declaredClasses {
		s.classesLru.Add(key, class)
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
	cls, ok := s.classesLru.Get(*classHash)
	if !ok {
		fmt.Printf("Unable to find class of hash %s\n", classHash.String())
		return s.base.Class(ctx, classHash)
	}

	return cls.(core.Class), nil
}

func (s *StartnetDataAdapter) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return s.p2p.GetStateUpdate(ctx, blockNumber)
}

// Typecheck
var _ starknetdata.StarknetData = &StartnetDataAdapter{}
