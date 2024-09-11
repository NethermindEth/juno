package p2p

import (
	"context"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/ethereum/go-ethereum/log"
)

type MockStarkData struct {
}

var _ starknetdata.StarknetData = (*MockStarkData)(nil)

func (m MockStarkData) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	log.Info("BlockByNumber", "blockNumber", blockNumber)
	return m.BlockLatest(ctx)
}

func (m MockStarkData) BlockLatest(ctx context.Context) (*core.Block, error) {
	// This is snapshot I have
	root, _ := (&felt.Felt{}).SetString("0x6df37678051ab529c243a5ae08e95eea4ddb40b874b4c537e2e6a9a459e2548")

	return &core.Block{
		Header: &core.Header{
			Number:          uint64(66489),
			GlobalStateRoot: root,
		},
	}, nil
}

func (m MockStarkData) BlockPending(ctx context.Context) (*core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockStarkData) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockStarkData) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockStarkData) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockStarkData) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockStarkData) StateUpdateWithBlock(ctx context.Context, blockNumber uint64) (*core.StateUpdate, *core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockStarkData) StateUpdatePendingWithBlock(ctx context.Context) (*core.StateUpdate, *core.Block, error) {
	//TODO implement me
	panic("implement me")
}
