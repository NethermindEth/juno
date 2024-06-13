package sync

import (
	"context"
	"os"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestSnapCopyTrie(t *testing.T) {
	var d db.DB
	d, _ = pebble.New("/home/amirul/fastworkscratch3/juno_db/juno_mainnet", 128000000, 128, utils.NewNopZapLogger())
	bc := blockchain.New(d, &utils.Mainnet) // Needed because class loader need encoder to be registered

	targetdir := "/home/amirul/fastworkscratch3/targetjuno"
	os.RemoveAll(targetdir)

	var d2 db.DB
	d2, _ = pebble.New(targetdir, 128000000, 128, utils.NewNopZapLogger())
	bc2 := blockchain.New(d2, &utils.Mainnet) // Needed because class loader need encoder to be registered

	logger, err := utils.NewZapLogger(utils.DEBUG, false)
	assert.NoError(t, err)

	syncer := NewSnapSyncer(
		&NoopService{
			blockchain: bc,
		},
		&localStarknetData{bc},
		&snapServer{
			blockchain: bc,
		},
		bc2,
		logger,
	)

	err = syncer.Run(context.Background())
	assert.NoError(t, err)
}

type NoopService struct {
	blockchain *blockchain.Blockchain
}

func (n NoopService) Run(ctx context.Context) error {
	return nil
}

type localStarknetData struct {
	blockchain *blockchain.Blockchain
}

func (l localStarknetData) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return l.blockchain.BlockByNumber(blockNumber)
}

func (l localStarknetData) BlockLatest(ctx context.Context) (*core.Block, error) {
	return l.blockchain.Head()
}

func (l localStarknetData) BlockPending(ctx context.Context) (*core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdateWithBlock(ctx context.Context, blockNumber uint64) (*core.StateUpdate, *core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdatePendingWithBlock(ctx context.Context) (*core.StateUpdate, *core.Block, error) {
	//TODO implement me
	panic("implement me")
}

var _ starknetdata.StarknetData = &localStarknetData{}
