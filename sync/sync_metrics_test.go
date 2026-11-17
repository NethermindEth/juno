package sync_test

import (
	"context"
	"errors"
	gosync "sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/adaptfeeder"
	"github.com/NethermindEth/juno/adapters/adaptfeeder/adaptfeedertest"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncReportsPipelineEvents(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Mainnet)
	gw := adaptfeeder.New(client)
	const lastBlock = 2

	bc := blockchain.New(
		memory.New(),
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)
	var (
		mu             gosync.Mutex
		ops            = make(map[string]map[uint64]bool)
		storedTxs      = make(map[uint64]int)
		classes        = make(map[uint64]int)
		started, ended int
	)
	feederSource := sync.NewFeederGatewayDataSource(bc, gw, sync.WithOnClassesFetched(
		func(blockNum uint64, count int, took time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			classes[blockNum] = count
		},
	))
	synchronizer := sync.New(
		bc,
		limitedDataSource{DataSource: feederSource, maxBlock: lastBlock},
		gw,
		log.NewNopZapLogger(),
		sync.WithPreConfirmed(preconfirmed.WithPollInterval(0)),
	)

	stored := make(chan struct{})
	synchronizer.WithListener(&sync.SelectiveListener{
		OnSyncStepDoneCb: func(op string, blockNum uint64, took time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			if ops[op] == nil {
				ops[op] = make(map[uint64]bool)
			}
			ops[op][blockNum] = true
		},
		OnFetchStartedCb: func() {
			mu.Lock()
			defer mu.Unlock()
			started++
		},
		OnFetchDoneCb: func() {
			mu.Lock()
			defer mu.Unlock()
			ended++
		},
		OnBlockStoredCb: func(blockNum uint64, txCount int) {
			mu.Lock()
			defer mu.Unlock()
			storedTxs[blockNum] = txCount
			if blockNum == lastBlock {
				close(stored)
			}
		},
	})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- synchronizer.Run(ctx)
	}()
	select {
	case <-stored:
	case <-time.After(timeout):
		t.Fatal("blocks were not stored")
	}
	cancel()
	require.NoError(t, <-done)

	pipelineOps := []string{
		sync.OpFetch, sync.OpFetchWait, sync.OpVerify, sync.OpVerifyWait, sync.OpStore,
	}
	mu.Lock()
	defer mu.Unlock()
	for blockNum := range uint64(lastBlock + 1) {
		block := adaptfeedertest.Block(t, client, blockNum)
		assert.Equal(t, len(block.Transactions), storedTxs[blockNum], "block %d", blockNum)
		assert.Contains(t, classes, blockNum, "block %d", blockNum)
		for _, op := range pipelineOps {
			assert.True(t, ops[op][blockNum], "op %s, block %d", op, blockNum)
		}
	}
	// Mainnet block 0 deploys contracts whose classes are not in the empty DB.
	assert.Positive(t, classes[0])
	// The fetch of the block after lastBlock never gets a block, so it reports no times.
	assert.NotContains(t, ops[sync.OpFetch], uint64(lastBlock+1))
	assert.NotContains(t, ops[sync.OpFetchWait], uint64(lastBlock+1))
	// Run waits for all fetches, so each started fetch has ended, the cancelled one too.
	assert.Greater(t, started, lastBlock+1)
	assert.Equal(t, started, ended)
}

// limitedDataSource serves only the blocks up to maxBlock.
type limitedDataSource struct {
	sync.DataSource
	maxBlock uint64
}

func (l limitedDataSource) BlockByNumber(
	ctx context.Context, blockNumber uint64,
) (sync.CommittedBlock, error) {
	if blockNumber > l.maxBlock {
		return sync.CommittedBlock{}, errors.New("block not available yet")
	}
	return l.DataSource.BlockByNumber(ctx, blockNumber)
}
