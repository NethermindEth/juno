package integtest

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type testConfig struct {
	// Number of nodes in the network
	nodeCount int
	// Number of faulty nodes in the network
	faultyNodeCount int
	// Buffer size for channels
	buffer int
	// Constant term of timeout function
	timeoutBase time.Duration
	// Step coefficient of timeout function
	timeoutStepFactor time.Duration
	// Round coefficient of timeout function
	timeoutRoundFactor time.Duration
	// Number of blocks to run the test for
	targetHeight types.Height
}

func getTimeoutFn(cfg testConfig) func(types.Step, types.Round) time.Duration {
	return func(step types.Step, round types.Round) time.Duration {
		return cfg.timeoutBase + time.Duration(step)*cfg.timeoutStepFactor + time.Duration(round)*cfg.timeoutRoundFactor
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
	honestNodeCount := cfg.nodeCount - cfg.faultyNodeCount

	allNodes := getNodes(cfg.nodeCount)
	network := newNetwork(allNodes, cfg.buffer)
	commits := make(chan commit, cfg.buffer)
	validators := newValidators(allNodes)

	// Spawn a network of honest nodes
	for i := range honestNodeCount {
		tendermintDB := newDB(t)

		nodeAddr := &allNodes.addr[i]

		stateMachine := tendermint.New(
			tendermintDB,
			utils.NewNopZapLogger(),
			*nodeAddr,
			&application{},
			validators,
			types.Height(0),
		)
		driver := driver.New(
			utils.NewNopZapLogger(),
			tendermintDB,
			stateMachine,
			newBlockchain(commits, nodeAddr),
			network.getListeners(nodeAddr),
			network.getBroadcasters(nodeAddr),
			getTimeoutFn(cfg),
		)

		driver.Start()
	}

	// node index -> height
	heights := make([]types.Height, honestNodeCount)
	// height -> committed value
	committedValues := make([]*starknet.Value, cfg.targetHeight)

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
		} else {
			// First committer, store the value
			committedValues[commit.height] = &commit.value
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
}

func TestTendermintCluster(t *testing.T) {
	t.Run("10 nodes, 200 blocks", func(t *testing.T) {
		runWithAllHonestAndSilentFaultyNodes(t, testConfig{
			nodeCount:          10,
			buffer:             100,
			timeoutBase:        10 * time.Millisecond,
			timeoutStepFactor:  10 * time.Millisecond,
			timeoutRoundFactor: 10 * time.Millisecond,
			targetHeight:       200,
		})
	})

	t.Run("100 nodes, 10 blocks", func(t *testing.T) {
		runWithAllHonestAndSilentFaultyNodes(t, testConfig{
			nodeCount:          100,
			buffer:             500,
			timeoutBase:        100 * time.Millisecond,
			timeoutStepFactor:  100 * time.Millisecond,
			timeoutRoundFactor: 100 * time.Millisecond,
			targetHeight:       10,
		})
	})
}
