package integ

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	nodeCount          int
	silentFaultyNodes  bool
	buffer             int
	timeoutBase        time.Duration
	timeoutStepFactor  time.Duration
	timeoutRoundFactor time.Duration
	targetHeight       types.Height
}

func getTimeoutFn(cfg testConfig) func(types.Step, types.Round) time.Duration {
	return func(step types.Step, round types.Round) time.Duration {
		return cfg.timeoutBase + time.Duration(step)*cfg.timeoutStepFactor + time.Duration(round)*cfg.timeoutRoundFactor
	}
}

func runTest(t *testing.T, cfg testConfig) {
	t.Helper()
	faultyNodeCount := (cfg.nodeCount - 1) / 3
	honestNodeCount := cfg.nodeCount - faultyNodeCount

	allNodes := getNodes(cfg.nodeCount)
	network := newNetwork(allNodes, cfg.buffer)
	commits := make(chan commit, cfg.buffer)
	validators := newValidators(allNodes)

	for i := range honestNodeCount {
		dbPath := t.TempDir()
		testDB, err := pebble.New(dbPath)
		require.NoError(t, err)

		nodeAddr := &allNodes.addr[i]

		stateMachine := tendermint.New(
			*nodeAddr,
			&application{},
			newBlockchain(commits, nodeAddr),
			validators,
		)
		driver := driver.New(
			testDB,
			stateMachine,
			network.getListeners(nodeAddr),
			network.getBroadcasters(nodeAddr),
			getTimeoutFn(cfg),
		)

		driver.Start()
	}

	// node index -> height
	heights := make([]types.Height, honestNodeCount)
	// height -> committed value
	committedValues := make([]*value, cfg.targetHeight)

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
		cfg.silentFaultyNodes = false
		runTest(t, cfg)
	})

	t.Run("silent faulty nodes", func(t *testing.T) {
		cfg := cfg
		cfg.silentFaultyNodes = true
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
