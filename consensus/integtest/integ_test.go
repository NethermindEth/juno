package integtest

import (
	"fmt"
	"os"
	"testing"
	"time"

	bc "github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/proposer"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/state_test_utils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

const (
	commitBufferSize   = 1024
	maxLag             = 10
	timeoutBase        = 500 * time.Microsecond
	timeoutRoundFactor = 100 * time.Millisecond
	gst                = 2 * time.Second // 2 Heartbeats should be enough for the nodes to connect to its peers
)

var (
	network  = utils.Mainnet
	logLevel = zap.DebugLevel
)

type testConfig struct {
	// Number of nodes in the network
	nodeCount int
	// Number of faulty nodes in the network
	faultyNodeCount int
	// Number of blocks to run the test for
	targetHeight types.Height
	// Network setup
	networkSetup networkConfigFn
}

func getTimeoutFn(cfg *testConfig) func(types.Step, types.Round) time.Duration {
	return func(step types.Step, round types.Round) time.Duration {
		// Total number of messages are N^2, so the load is roughly proportional to O(N^2)
		// Every round increases the timeout by timeoutRoundFactor. It also guarantees that the timeout will be at least timeoutRoundFactor
		delta := time.Duration(cfg.nodeCount*cfg.nodeCount)*timeoutBase + time.Duration(round+1)*timeoutRoundFactor

		// The formulae follow the lemma in the paper
		switch step {
		case types.StepPropose:
			prevDelta := delta - timeoutRoundFactor
			return (prevDelta + delta) * 2
		case types.StepPrevote:
			return delta * 2
		case types.StepPrecommit:
			return delta * 2
		}
		return 0
	}
}

func TestMain(m *testing.M) {
	statetestutils.Parse()
	os.Exit(m.Run())
}

func newDB(t *testing.T) *mocks.MockTendermintDB[starknet.Value, starknet.Hash, starknet.Address] {
	t.Helper()
	ctrl := gomock.NewController(t)
	// Ignore WAL for tests that use this
	db := mocks.NewMockTendermintDB[starknet.Value, starknet.Hash, starknet.Address](ctrl)
	db.EXPECT().GetWALEntries(gomock.Any()).AnyTimes()
	db.EXPECT().SetWALEntry(gomock.Any()).AnyTimes()
	db.EXPECT().Flush().AnyTimes()
	db.EXPECT().DeleteWALEntries(gomock.Any()).AnyTimes()
	return db
}

func getBuilder(t *testing.T, genesisDiff core.StateDiff, genesisClasses map[felt.Felt]core.Class) *builder.Builder {
	t.Helper()
	testDB := memory.New()
	network := &utils.Mainnet
	log := utils.NewNopZapLogger()

	bc := bc.New(testDB, network, statetestutils.UseNewState())
	require.NoError(t, bc.StoreGenesis(&genesisDiff, genesisClasses))

	executor := builder.NewExecutor(bc, vm.New(false, log), log, false, true)
	testBuilder := builder.New(bc, executor)
	return &testBuilder
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

func toValue(value *felt.Felt) starknet.Value {
	return starknet.Value(*value)
}

func initNode(
	t *testing.T,
	gstChan chan struct{},
	nodes *nodes,
	p2pHost *networkNodeConfig,
	logger *utils.ZapLogger,
	commits chan commit,
	cfg *testConfig,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.Class,
) {
	t.Helper()
	defer func() {
		logger.Debug("finished initNode")
	}()

	nodeAddr := testAddress(p2pHost.index)
	logger = &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprint(p2pHost.index))}
	tendermintDB := newDB(t)
	builder := getBuilder(t, genesisDiff, genesisClasses)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	proposer := proposer.New(logger, builder, &proposalStore, nodeAddr, toValue)
	stateMachine := tendermint.New(tendermintDB, logger, nodeAddr, proposer, nodes, types.Height(0))
	p2p := p2p.New(p2pHost.host, logger, builder, &proposalStore, types.Height(0), &config.DefaultBufferSizes)
	bc := newBlockchain(commits, &nodeAddr)
	driver := driver.New(logger, tendermintDB, stateMachine, bc, p2p, proposer, getTimeoutFn(cfg))

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		require.NoError(t, proposer.Run(t.Context()))
	})
	wg.Go(func() {
		require.NoError(t, p2p.Run(t.Context()))
	})
	wg.Go(func() {
		<-gstChan
		require.NoError(t, driver.Run(t.Context()))
	})
	t.Cleanup(wg.Wait)
}

func assertCommits(t *testing.T, commits chan commit, cfg testConfig, logger *utils.ZapLogger) {
	t.Helper()

	honestNodeCount := cfg.nodeCount - cfg.faultyNodeCount

	// node index -> height. This is to verify that the nodes reach the target height.
	heights := make([]types.Height, honestNodeCount)
	// height -> committed value. This is to verify that the nodes reach consensus.
	committedValues := make([]*starknet.Value, cfg.targetHeight)
	// height -> number of nodes that committed at this height. This is for debugging purposes.
	commitCount := make(map[types.Height]int)

	nextHeight := types.Height(0)
	for commit := range commits {
		// Ignore commits after the target height
		if commit.height >= cfg.targetHeight {
			continue
		}

		require.Falsef(
			t,
			commit.height > nextHeight+maxLag,
			"finished height %d is too far behind committed height %d",
			nextHeight,
			commit.height,
		)

		// Ignore faulty nodes
		nodeIdx := testAddressIndex(commit.nodeAddr)
		if nodeIdx >= honestNodeCount {
			continue
		}

		// Must commit in order
		assert.Equal(t, heights[nodeIdx], commit.height)
		heights[nodeIdx]++

		// If subsequent committer, check the value, otherwise store the value
		if value := committedValues[commit.height]; value != nil {
			assert.Equal(t, *value, commit.value)
		} else {
			committedValues[commit.height] = &commit.value
		}

		// Count the number of nodes that committed at this height
		commitCount[commit.height]++

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

	allNodes := newNodes(cfg.nodeCount)
	commits := make(chan commit, commitBufferSize)

	p2pHosts := cfg.networkSetup(t, honestNodeCount)
	p2pHosts.setup(t)

	gstChan := make(chan struct{})

	iterator := iter.Iterator[networkNodeConfig]{MaxGoroutines: honestNodeCount}
	iterator.ForEach(p2pHosts, func(p2pHost *networkNodeConfig) {
		initNode(t, gstChan, &allNodes, p2pHost, logger, commits, &cfg, genesisDiff, genesisClasses)
	})

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
