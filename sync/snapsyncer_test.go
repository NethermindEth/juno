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
	d, _ = pebble.New("/home/amirul/fastworkscratch/largejuno_goerli", utils.NewNopZapLogger())
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	targetdir := "/home/amirul/fastworkscratch3/targetjuno"
	os.RemoveAll(targetdir)

	var d2 db.DB
	d2, _ = pebble.New(targetdir, utils.NewNopZapLogger())
	bc2 := blockchain.New(d2, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	logger, err := utils.NewZapLogger(utils.DEBUG, false)
	assert.NoError(t, err)

	syncer := NewSnapSyncer(
		&NoopService{},
		&localStarknetData{bc},
		bc,
		bc2,
		logger,
	)

	err = syncer.Run(context.Background())
	assert.NoError(t, err)
}

type NoopService struct {
}

func (n NoopService) Run(ctx context.Context) error {
	return nil
}

type localStarknetData struct {
	blockchain *blockchain.Blockchain
}

func (n *localStarknetData) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return n.blockchain.BlockByNumber(blockNumber)
}

func (n *localStarknetData) BlockLatest(ctx context.Context) (*core.Block, error) {
	return n.blockchain.Head()
}

func (n *localStarknetData) BlockPending(ctx context.Context) (*core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (n *localStarknetData) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (n *localStarknetData) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	//TODO implement me
	panic("implement me")
}

func (n *localStarknetData) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return n.blockchain.StateUpdateByNumber(blockNumber)
}

func (n *localStarknetData) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

var _ starknetdata.StarknetData = &localStarknetData{}
