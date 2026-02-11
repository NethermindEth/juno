package sync_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	waitInterval = 10 * time.Millisecond
	waitTimeout  = 5 * time.Second
	logLevel     = zap.PanicLevel
)

var network = &utils.Mainnet

type testBlockDataSource atomic.Value

func newTestBlockDataSource() *testBlockDataSource {
	source := testBlockDataSource{}
	source.setBlocks([]sync.CommittedBlock{})
	return &source
}

func (t *testBlockDataSource) BlockByNumber(ctx context.Context, blockNumber uint64) (sync.CommittedBlock, error) {
	blocks := t.getBlocks()
	if blockNumber >= uint64(len(blocks)) {
		return sync.CommittedBlock{}, errors.New("block not found")
	}

	return getBlock(blocks, blockNumber), nil
}

func (t *testBlockDataSource) BlockLatest(ctx context.Context) (*core.Block, error) {
	blocks := t.getBlocks()
	if len(blocks) == 0 {
		return nil, errors.New("no blocks")
	}

	return getBlock(blocks, uint64(len(blocks)-1)).Block, nil
}

func (t *testBlockDataSource) BlockPending(ctx context.Context) (core.Pending, error) {
	return core.Pending{}, errors.New("not implemented")
}

func (t *testBlockDataSource) PreConfirmedBlockByNumber(ctx context.Context, blockNumber uint64) (core.PreConfirmed, error) {
	return core.PreConfirmed{}, errors.New("not implemented")
}

func (t *testBlockDataSource) setBlocks(blocks []sync.CommittedBlock) {
	(*atomic.Value)(t).Store(blocks)
}

func (t *testBlockDataSource) getBlocks() []sync.CommittedBlock {
	return (*atomic.Value)(t).Load().([]sync.CommittedBlock)
}

func getBlock(blocks []sync.CommittedBlock, blockNumber uint64) sync.CommittedBlock {
	committedBlock := blocks[blockNumber]
	committedBlock.Persisted = make(chan struct{})
	return committedBlock
}

type blockGenerator struct {
	blocks     []sync.CommittedBlock
	database   *memory.Database
	blockchain *blockchain.Blockchain
	builder    *builder.Builder
	sequencer  uint64
}

func initGenesis(t *testing.T) (*memory.Database, sync.CommittedBlock) {
	t.Helper()

	database := memory.New()
	bc := blockchain.New(database, network, statetestutils.UseNewState())

	genesisConfig, err := genesis.Read("../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../genesis/classes/strk.json", "../genesis/classes/account.json",
		"../genesis/classes/universaldeployer.json", "../genesis/classes/udacnt.json",
	}

	feeTokens := utils.DefaultFeeTokenAddresses
	chainInfo := vm.ChainInfo{
		ChainID:           network.L2ChainID,
		FeeTokenAddresses: feeTokens,
	}
	diff, classes, err := genesis.GenesisStateDiff(
		genesisConfig,
		vm.New(&chainInfo, false, utils.NewNopZapLogger()),
		bc.Network(),
		vm.DefaultMaxGas,
		vm.DefaultMaxGas,
	)
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))

	block, err := bc.BlockByNumber(0)
	require.NoError(t, err)

	stateUpdate, err := bc.StateUpdateByNumber(0)
	require.NoError(t, err)

	committedBlock := sync.CommittedBlock{
		Block:       block,
		StateUpdate: stateUpdate,
		NewClasses:  classes,
	}
	return database, committedBlock
}

func newBlockGenerator(
	blocks []sync.CommittedBlock,
	database *memory.Database,
	sequencer uint64,
) *blockGenerator {
	bc := blockchain.New(database, network, statetestutils.UseNewState())
	builder := newTestBuilder(utils.NewNopZapLogger(), bc)

	return &blockGenerator{
		blocks:     blocks,
		database:   database,
		blockchain: bc,
		builder:    builder,
		sequencer:  sequencer,
	}
}

func (b *blockGenerator) clone() *blockGenerator {
	return newBlockGenerator(slices.Clone(b.blocks), b.database.Copy(), b.sequencer+1)
}

func (b *blockGenerator) mine(t *testing.T, dataSource *testBlockDataSource, count int) {
	t.Helper()
	sequencer := new(felt.Felt).SetUint64(b.sequencer)
	buildParams := newTestBuildParams(sequencer)
	for range count {
		buildState, err := b.builder.InitPreconfirmedBlock(&buildParams)
		require.NoError(t, err)

		buildResult, err := b.builder.Finish(buildState)
		require.NoError(t, err)

		committedBlock := sync.CommittedBlock{
			Block:       buildResult.Preconfirmed.Block,
			StateUpdate: buildResult.Preconfirmed.StateUpdate,
			NewClasses:  buildResult.Preconfirmed.NewClasses,
		}

		b.blocks = append(b.blocks, committedBlock)

		commitments, err := b.blockchain.SanityCheckNewHeight(
			committedBlock.Block,
			committedBlock.StateUpdate,
			committedBlock.NewClasses,
		)
		require.NoError(t, err)

		err = b.blockchain.Store(
			committedBlock.Block,
			commitments,
			committedBlock.StateUpdate,
			committedBlock.NewClasses,
		)
		require.NoError(t, err)
	}

	dataSource.setBlocks(b.blocks)
}

func newTestBuilder(log utils.Logger, bc *blockchain.Blockchain) *builder.Builder {
	feeTokens := utils.DefaultFeeTokenAddresses
	chainInfo := vm.ChainInfo{
		ChainID:           network.L2ChainID,
		FeeTokenAddresses: feeTokens,
	}
	executor := builder.NewExecutor(bc, vm.New(&chainInfo, false, log), log, false, true)
	builder := builder.New(bc, executor)
	return &builder
}

func newTestBuildParams(sequencer *felt.Felt) builder.BuildParams {
	return builder.BuildParams{
		Builder:           *sequencer,
		Timestamp:         uint64(time.Now().Unix()),
		L2GasPriceFRI:     felt.One,  // TODO: Implement this properly
		L1GasPriceWEI:     felt.One,  // TODO: Implement this properly
		L1DataGasPriceWEI: felt.One,  // TODO: Implement this properly
		EthToStrkRate:     felt.One,  // TODO: Implement this properly
		L1DAMode:          core.Blob, // TODO: Implement this properly
	}
}

func setup(
	t *testing.T,
	genesisDatabase *memory.Database,
	genesisBlock sync.CommittedBlock,
) (*blockGenerator, *testBlockDataSource, *blockchain.Blockchain) {
	t.Helper()

	blockGeneratorDatabase := genesisDatabase.Copy()
	synchronizerDatabase := genesisDatabase.Copy()

	dataSource := newTestBlockDataSource()

	blockGenerator := newBlockGenerator(
		[]sync.CommittedBlock{genesisBlock},
		blockGeneratorDatabase,
		0,
	)
	blockGenerator.mine(t, dataSource, 10)

	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), false)
	require.NoError(t, err)

	blockchain := blockchain.New(synchronizerDatabase, network, statetestutils.UseNewState())

	wg := gosync.WaitGroup{}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(wg.Wait)
	t.Cleanup(cancel)
	wg.Go(func() {
		synchronizer := sync.New(blockchain, dataSource, logger, 0, 0, false, synchronizerDatabase)
		require.NoError(t, synchronizer.Run(ctx))
	})

	return blockGenerator, dataSource, blockchain
}

func requireBlocks(t *testing.T, blockchain *blockchain.Blockchain, blocks []sync.CommittedBlock) {
	t.Helper()
	require.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			expectedHeight := uint64(len(blocks) - 1)
			height, err := blockchain.Height()
			require.NoError(c, err)
			require.Equal(c, expectedHeight, height)

			for i := range blocks {
				block, err := blockchain.BlockByNumber(uint64(i))
				require.NoError(c, err)
				require.Equal(c, blocks[i].Block.Header, block.Header)

				stateUpdate, err := blockchain.StateUpdateByNumber(uint64(i))
				require.NoError(c, err)
				require.Equal(c, blocks[i].StateUpdate, stateUpdate)
			}
		},
		waitTimeout,
		waitInterval,
	)
}

func testReorgWithImmediateReplacement(
	t *testing.T,
	genesisDatabase *memory.Database,
	genesisBlock sync.CommittedBlock,
	localLength,
	forkLength int,
) {
	t.Helper()

	oldFork, dataSource, blockchain := setup(t, genesisDatabase, genesisBlock)

	t.Run("First batch is synced", func(t *testing.T) {
		requireBlocks(t, blockchain, oldFork.blocks)
	})

	newFork := oldFork.clone()

	t.Run(fmt.Sprintf("Mine %d more blocks", localLength), func(t *testing.T) {
		oldFork.mine(t, dataSource, localLength)
		requireBlocks(t, blockchain, oldFork.blocks)
	})

	t.Run(fmt.Sprintf("Fork mine %d more blocks", forkLength), func(t *testing.T) {
		newFork.mine(t, dataSource, forkLength)
		requireBlocks(t, blockchain, newFork.blocks)
	})
}

func TestReorgDetection(t *testing.T) {
	genesisDatabase, genesisBlock := initGenesis(t)
	t.Run("Detect reorg when cannot fetch new block", func(t *testing.T) {
		oldFork, dataSource, blockchain := setup(t, genesisDatabase, genesisBlock)

		t.Run("First batch is synced", func(t *testing.T) {
			requireBlocks(t, blockchain, oldFork.blocks)
		})

		newFork := oldFork.clone()

		t.Run("Mine 5 more blocks", func(t *testing.T) {
			oldFork.mine(t, dataSource, 5)
			requireBlocks(t, blockchain, oldFork.blocks)
		})

		t.Run("Remove last 5 blocks, avoid reorg immediately", func(t *testing.T) {
			dataSource.setBlocks(newFork.blocks)
			requireBlocks(t, blockchain, oldFork.blocks)
		})

		t.Run("Add back 5 blocks, nothing changed", func(t *testing.T) {
			dataSource.setBlocks(oldFork.blocks)
			requireBlocks(t, blockchain, oldFork.blocks)
		})

		t.Run("Roll back 5 blocks and mine 2 blocks from fork", func(t *testing.T) {
			newFork.mine(t, dataSource, 2)
			requireBlocks(t, blockchain, newFork.blocks)
		})
	})

	t.Run("Fork has the same height as local chain", func(t *testing.T) {
		testReorgWithImmediateReplacement(t, genesisDatabase, genesisBlock, 5, 5)
	})

	t.Run("Fork is longer than local chain", func(t *testing.T) {
		testReorgWithImmediateReplacement(t, genesisDatabase, genesisBlock, 5, 10)
	})

	t.Run("Fork is shorter than local chain", func(t *testing.T) {
		testReorgWithImmediateReplacement(t, genesisDatabase, genesisBlock, 10, 5)
	})
}
