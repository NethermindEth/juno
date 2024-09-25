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
	root, _ := (&felt.Felt{}).SetString("0x472e84b65d387c9364b5117f4afaba3fb88897db1f28867b398506e2af89f25")

	return &core.Block{
		Header: &core.Header{
			Number:          uint64(66477),
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
