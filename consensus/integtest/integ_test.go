package integtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/state_test_utils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	commitBufferSize = 1024
	maxLag           = 10
	gst              = 2 * time.Second // 2 Heartbeats should be enough for the nodes to connect to its peers
	hostAddress      = "/ip4/0.0.0.0/tcp/0"
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
	networkSetup networkConfigFn
}

func getBlockchain(t *testing.T, genesisDiff core.StateDiff, genesisClasses map[felt.Felt]core.Class) *blockchain.Blockchain {
	t.Helper()
	testDB := memory.New()
	network := &utils.Mainnet
	log := utils.NewNopZapLogger()

	bc := bc.New(testDB, network, statetestutils.UseNewState())
	require.NoError(t, bc.StoreGenesis(&genesisDiff, genesisClasses))
	return bc
}

func loadGenesis(t *testing.T, log *utils.ZapLogger) (core.StateDiff, map[felt.Felt]core.Class) {
	t.Helper()
	genesisConfig, err := genesis.Read("../../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)

	genesisConfig.Classes = []string{
		"../../genesis/classes/strk.json", "../../genesis/classes/account.json",
		"../../genesis/classes/universaldeployer.json", "../../genesis/classes/udacnt.json",
	}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), &network, 40000000) //nolint:gomnd
	require.NoError(t, err)

	return diff, classes
}

func initNode(
	t *testing.T,
	gstChan chan struct{},
	index int,
	logger *utils.ZapLogger,
	commits chan commit,
	cfg *testConfig,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.Class,
) host.Host {
	t.Helper()
	defer func() {
		logger.Debug("finished initNode")
	}()

	mockServices := consensus.InitMockServices(0, 0, index, cfg.nodeCount)

	logger = &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprint(index))}
	consensusDB := memory.New()
	bc := getBlockchain(t, genesisDiff, genesisClasses)
	vm := vm.New(false, logger)

	services, err := consensus.Init(
		logger,
		consensusDB,
		bc,
		vm,
		&mockServices.NodeAddress,
		mockServices.Validators,
		mockServices.TimeoutFn,
		hostAddress,
		mockServices.PrivateKey,
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
		<-gstChan
		require.NoError(t, services.Driver.Run(t.Context()))
	})
	wg.Go(func() {
		writeBlock(t, index, bc, services.CommitListener, commits)
	})
	t.Cleanup(wg.Wait)

	return services.Host
}

func writeBlock(
	t *testing.T,
	index int,
	bc *blockchain.Blockchain,
	commitListener driver.CommitListener[starknet.Value, starknet.Hash, starknet.Address],
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
	for commit := range commits {
		blockNumber := commit.committedBlock.Block.Number
		blockHash := commit.committedBlock.Block.Hash
		// Ignore commits after the target height
		if blockNumber > cfg.targetHeight {
			continue
		}

		require.Falsef(
			t,
			blockNumber > nextHeight+maxLag,
			"finished height %d is too far behind committed height %d",
			nextHeight,
			blockNumber,
		)

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
		for commitCount[nextHeight] == honestNodeCount && nextHeight < cfg.targetHeight {
			logger.Infow("all honest nodes committed", "height", nextHeight)
			nextHeight++
		}

		if nextHeight == cfg.targetHeight {
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

	gstChan := make(chan struct{})

	hosts := make(networkConfig, honestNodeCount)
	iterator := iter.Iterator[networkNodeConfig]{MaxGoroutines: honestNodeCount}
	iterator.ForEachIdx(hosts, func(i int, nodeConfig *networkNodeConfig) {
		*nodeConfig = networkNodeConfig{
			host:          initNode(t, gstChan, i, logger, commits, &cfg, genesisDiff, genesisClasses),
			adjacentNodes: make(map[int]struct{}),
		}
	})
	hosts.setup(t, cfg.networkSetup)

	time.Sleep(gst)
	close(gstChan)

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
		networkSetup: lineNetworkConfig,
	})

	runWithAllHonestAndSilentFaultyNodes(t, testConfig{
		nodeCount:    20,
		targetHeight: 60,
		networkSetup: smallWorldNetworkConfig,
	})

	runWithAllHonestAndSilentFaultyNodes(t, testConfig{
		nodeCount:    50,
		targetHeight: 15,
		networkSetup: smallWorldNetworkConfig,
	})
}
