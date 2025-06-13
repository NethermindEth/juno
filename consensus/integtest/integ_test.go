package integtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

const (
	commitBufferSize   = 1024
	timeoutBase        = 10 * time.Microsecond
	timeoutRoundFactor = 20 * time.Millisecond
)

var logLevel = zap.InfoLevel

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

func getTimeoutFn(cfg testConfig) func(types.Step, types.Round) time.Duration {
	return func(step types.Step, round types.Round) time.Duration {
		// Total number of messages are N^2 and we're running N nodes, so the load is roughly proportional to O(N^3)
		// Every round increases the timeout by timeoutRoundFactor. It also guarantees that the timeout will be at least timeoutRoundFactor
		delta := time.Duration(cfg.nodeCount*cfg.nodeCount*cfg.nodeCount)*timeoutBase + time.Duration(round+1)*timeoutRoundFactor

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

func runTest(t *testing.T, cfg testConfig) {
	t.Helper()
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	honestNodeCount := cfg.nodeCount - cfg.faultyNodeCount

	allNodes := getNodes(cfg.nodeCount)
	commits := make(chan commit, commitBufferSize)
	validators := newValidators(allNodes)

	p2pWaitGroup := conc.NewWaitGroup()
	t.Cleanup(func() {
		p2pWaitGroup.Wait()
	})

	p2pHosts := cfg.networkSetup(t, honestNodeCount)

	drivers := make([]driver.Driver[starknet.Value, starknet.Hash, starknet.Address], honestNodeCount)
	t.Cleanup(func() {
		for i := range drivers {
			drivers[i].Stop()
		}
	})

	// Spawn a network of honest nodes
	for i := range honestNodeCount {
		logger := &utils.ZapLogger{SugaredLogger: logger.Named(fmt.Sprint(i))}
		tendermintDB := newDB(t)

		nodeAddr := &allNodes.addr[i]

		stateMachine := tendermint.New(
			tendermintDB,
			logger,
			*nodeAddr,
			&application{},
			validators,
			types.Height(0),
		)

		p2p := p2p.New(p2pHosts[i].host, logger, types.Height(0), &config.DefaultBufferSizes)
		p2pWaitGroup.Go(func() {
			p2pHosts[i].setupDiscovery()
			require.NoError(t, p2p.Run(t.Context()))
		})

		drivers[i] = driver.New(
			logger,
			tendermintDB,
			stateMachine,
			newBlockchain(commits, nodeAddr),
			p2p,
			getTimeoutFn(cfg),
		)
	}

	// Wait for all nodes to be ready. This is to simulate the GST.
	for i := range p2pHosts {
		<-p2pHosts[i].ready
	}

	for i := range drivers {
		drivers[i].Start()
	}

	// node index -> height. This is to verify that the nodes reach the target height.
	heights := make([]types.Height, honestNodeCount)
	// height -> committed value. This is to verify that the nodes reach consensus.
	committedValues := make([]*starknet.Value, cfg.targetHeight)
	// height -> number of nodes that committed at this height. This is for debugging purposes.
	commitCount := make(map[types.Height]int)

	finished := 0
	for commit := range commits {
		// Ignore commits after the target height
		if commit.height >= cfg.targetHeight {
			continue
		}

		nodeIdx, ok := allNodes.index[*commit.nodeAddr]
		assert.True(t, ok)

		// Ignore faulty nodes
		if nodeIdx >= honestNodeCount {
			continue
		}

		// Must commit in order
		assert.Equal(t, heights[nodeIdx], commit.height)
		heights[nodeIdx]++

		if value := committedValues[commit.height]; value != nil {
			// Subsequent committer, check the value
			assert.Equal(t, *value, commit.value)
			commitCount[commit.height]++
			if commitCount[commit.height] == honestNodeCount {
				logger.Infow("all honest nodes committed", "height", commit.height)
			}
		} else {
			// First committer, store the value
			committedValues[commit.height] = &commit.value
			commitCount[commit.height] = 1
		}

		if commit.height == cfg.targetHeight-1 {
			finished++
		}
		if finished == honestNodeCount {
			break
		}
	}
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
		targetHeight: 10,
		networkSetup: smallWorldNetworkConfig,
	})
}
