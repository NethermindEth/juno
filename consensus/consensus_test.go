package consensus_test

import (
	"fmt"
	goitre "iter"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/p2p/pubsub/testutils"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	commitBufferSize = 1024
	maxRoundWait     = types.Round(5)
)

var (
	network  = utils.Mainnet
	logLevel = zap.DebugLevel
)

type commit struct {
	nodeIndex      int
	committedBlock sync.CommittedBlock
}

type testConfig struct {
	// Number of nodes in the network
	nodeCount int
	// Number of faulty nodes in the network
	faultyNodeCount int
	// Number of blocks to run the test for
	targetHeight uint64
	// Network setup
	networkSetup testutils.NetworkConfigFn
}

func getBlockchain(
	t *testing.T,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.ClassDefinition,
) *blockchain.Blockchain {
	t.Helper()
	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network)
	require.NoError(t, bc.StoreGenesis(&genesisDiff, genesisClasses))
	return bc
}

func loadGenesis(
	t *testing.T,
	log *utils.ZapLogger,
) (core.StateDiff, map[felt.Felt]core.ClassDefinition) {
	t.Helper()
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
		vm.New(&chainInfo, false, log),
		&network,
		vm.DefaultMaxSteps,
		vm.DefaultMaxGas,
	)
	require.NoError(t, err)

	return diff, classes
}

func initNode(
	t *testing.T,
	index int,
	node *testutils.Node,
	logger *utils.ZapLogger,
	commits chan commit,
	cfg *testConfig,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.ClassDefinition,
) {
	t.Helper()

	mockServices := consensus.InitMockServices(0, 0, index, cfg.nodeCount)

	logger = &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprint(index))}
	consensusDB := memory.New()
	bc := getBlockchain(t, genesisDiff, genesisClasses)

	feeTokens := utils.DefaultFeeTokenAddresses
	chainInfo := vm.ChainInfo{
		ChainID:           network.L2ChainID,
		FeeTokenAddresses: feeTokens,
	}
	vm := vm.New(&chainInfo, false, logger)

	services, err := consensus.Init(
		node.Host,
		logger,
		consensusDB,
		bc,
		vm,
		&mockServices.NodeAddress,
		mockServices.Validators,
		mockServices.TimeoutFn,
		node.GetBootstrapPeers,
	)
	require.NoError(t, err)

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		require.NoError(t, services.Proposer.Run(t.Context()))
	})
	wg.Go(func() {
		require.NoError(t, services.P2P.Run(t.Context()))
	})
	wg.Go(func() {
		require.NoError(t, services.Driver.Run(t.Context()))
	})
	wg.Go(func() {
		writeBlock(t, index, bc, services.CommitListener, commits)
	})
	t.Cleanup(wg.Wait)
}

func writeBlock(
	t *testing.T,
	index int,
	bc *blockchain.Blockchain,
	commitListener driver.CommitListener[starknet.Value, starknet.Hash],
	commits chan commit,
) {
	t.Helper()
	for {
		select {
		case <-t.Context().Done():
			return
		case committedBlock := <-commitListener.Listen():
			commitments, err := bc.SanityCheckNewHeight(committedBlock.Block, committedBlock.StateUpdate, committedBlock.NewClasses)
			require.NoError(t, err)
			require.NoError(t, bc.Store(committedBlock.Block, commitments, committedBlock.StateUpdate, committedBlock.NewClasses))

			close(committedBlock.Persisted)

			commit := commit{
				nodeIndex:      index,
				committedBlock: committedBlock,
			}
			select {
			case <-t.Context().Done():
				return
			case commits <- commit:
			}
		}
	}
}

func commitStream(t *testing.T, nodeCount int, commits chan commit) goitre.Seq[commit] {
	t.Helper()

	timeoutFn := consensus.MockTimeoutFn(nodeCount)
	var maxCommitWait time.Duration
	for round := range maxRoundWait {
		maxCommitWait += timeoutFn(types.StepPropose, round) + timeoutFn(types.StepPrevote, round) + timeoutFn(types.StepPrecommit, round)
	}
	return func(yield func(commit) bool) {
		t.Helper()
		for {
			select {
			case commit := <-commits:
				if !yield(commit) {
					return
				}
			case <-time.After(maxCommitWait):
				require.FailNow(t, "timed out waiting for commit")
			}
		}
	}
}

func assertCommits(t *testing.T, commits chan commit, cfg testConfig, logger *utils.ZapLogger) {
	t.Helper()

	honestNodeCount := cfg.nodeCount - cfg.faultyNodeCount

	// node index -> height. This is to verify that the nodes reach the target height.
	heights := make([]uint64, honestNodeCount)
	// height -> committed value. This is to verify that the nodes reach consensus.
	committedValues := make([]*felt.Felt, cfg.targetHeight+1)
	// height -> number of nodes that committed at this height. This is for debugging purposes.
	commitCount := make(map[uint64]int)

	nextHeight := uint64(1)
	for commit := range commitStream(t, cfg.nodeCount, commits) {
		blockNumber := commit.committedBlock.Block.Number
		blockHash := commit.committedBlock.Block.Hash

		require.Falsef(
			t,
			blockNumber > nextHeight+cfg.targetHeight,
			"finished height %d is too far behind committed height %d",
			nextHeight,
			blockNumber,
		)

		// Ignore commits after the target height
		if blockNumber > cfg.targetHeight {
			continue
		}

		// Ignore faulty nodes
		if commit.nodeIndex >= honestNodeCount {
			continue
		}

		// Must commit in order
		heights[commit.nodeIndex]++
		assert.Equal(t, heights[commit.nodeIndex], blockNumber)

		// If subsequent committer, check the value, otherwise store the value
		if value := committedValues[blockNumber]; value != nil {
			assert.Equal(t, value, blockHash)
		} else {
			committedValues[blockNumber] = blockHash
		}

		// Count the number of nodes that committed at this height
		commitCount[blockNumber]++

		// If all honest nodes committed at this height, increment the finished counter
		for commitCount[nextHeight] == honestNodeCount && nextHeight <= cfg.targetHeight {
			logger.Infow("all honest nodes committed", "height", nextHeight)
			nextHeight++
		}

		if nextHeight > cfg.targetHeight {
			break
		}
	}
}

func runTest(t *testing.T, cfg testConfig) {
	t.Helper()
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	genesisDiff, genesisClasses := loadGenesis(t, logger)

	honestNodeCount := cfg.nodeCount - cfg.faultyNodeCount

	commits := make(chan commit, commitBufferSize)

	nodes := testutils.BuildNetworks(t, cfg.networkSetup(honestNodeCount))

	iterator := iter.Iterator[testutils.Node]{MaxGoroutines: honestNodeCount}
	iterator.ForEachIdx(nodes, func(i int, node *testutils.Node) {
		initNode(t, i, node, logger, commits, &cfg, genesisDiff, genesisClasses)
	})

	assertCommits(t, commits, cfg, logger)
}

func runWithAllHonestAndSilentFaultyNodes(t *testing.T, cfg testConfig) {
	t.Helper()
	t.Run(fmt.Sprintf("%d nodes, %d blocks", cfg.nodeCount, cfg.targetHeight), func(t *testing.T) {
		t.Run("all honest", func(t *testing.T) {
			cfg := cfg
			cfg.faultyNodeCount = 0
			runTest(t, cfg)
		})

		t.Run("silent faulty nodes", func(t *testing.T) {
			cfg := cfg
			cfg.faultyNodeCount = (cfg.nodeCount - 1) / 3
			runTest(t, cfg)
		})
	})
}

func TestTendermintCluster(t *testing.T) {
	runWithAllHonestAndSilentFaultyNodes(t, testConfig{
		nodeCount:    4,
		targetHeight: 10,
		networkSetup: testutils.LineNetworkConfig,
	})

	runWithAllHonestAndSilentFaultyNodes(t, testConfig{
		nodeCount:    20,
		targetHeight: 60,
		networkSetup: testutils.SmallWorldNetworkConfig,
	})

	runWithAllHonestAndSilentFaultyNodes(t, testConfig{
		nodeCount:    50,
		targetHeight: 15,
		networkSetup: testutils.SmallWorldNetworkConfig,
	})
}
