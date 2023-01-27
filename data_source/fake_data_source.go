package datasource

import (
	"errors"
	"math/rand"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type FakeDataSource struct{}

func (f *FakeDataSource) GetBlockByNumber(blockNumber uint64) (*core.Block, error) {
	if rand.Float64() < 0.33 {
		return nil, errors.New("come back later")
	}

	return &core.Block{
		Number: blockNumber,
	}, nil
}

func (f *FakeDataSource) GetTransaction(transactionHash *felt.Felt) (*core.Transaction, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeDataSource) GetClass(classHash *felt.Felt) (*core.Class, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeDataSource) GetStateUpdate(blockNumber uint64) (*core.StateUpdate, error) {
	if rand.Float64() < 0.33 {
		return nil, errors.New("come back later")
	}

	return &core.StateUpdate{
		BlockHash: new(felt.Felt).SetUint64(blockNumber),
	}, nil
}
