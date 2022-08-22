package sync

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	blockdb "github.com/NethermindEth/juno/internal/db/block"
	syncdb "github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
)

type mockStateDiffCollector struct {
	latestBlock *feeder.StarknetBlock
}

// Unimplemented right now
func (m *mockStateDiffCollector) PendingBlock() *feeder.StarknetBlock { return nil }
func (m *mockStateDiffCollector) Close()                              {}
func (m *mockStateDiffCollector) Run()                                {}
func (m *mockStateDiffCollector) GetChannel() chan *CollectorDiff     { return nil }

func (m *mockStateDiffCollector) LatestBlock() *feeder.StarknetBlock {
	return m.latestBlock
}

func TestStatus(t *testing.T) {
	mdbxEnv, err := db.NewMDBXEnv(t.TempDir(), 100, 0)
	if err != nil {
		t.Errorf("could not set up database environment: %s", err)
	}

	syncDb, err := db.NewMDBXDatabase(mdbxEnv, "SYNC")
	if err != nil {
		t.Errorf("could not set up database: %s", err)
	}
	blockDb, err := db.NewMDBXDatabase(mdbxEnv, "BLOCK")
	if err != nil {
		t.Errorf("could not set up database: %s", err)
	}

	blockNumber := int64(4)

	m := &mockStateDiffCollector{
		latestBlock: &feeder.StarknetBlock{
			BlockHash:   "123",
			BlockNumber: feeder.BlockNumber(blockNumber + 1),
		},
	}
	s := &Synchronizer{
		syncManager:             syncdb.NewManager(syncDb),
		blockManager:            blockdb.NewManager(blockDb),
		latestBlockNumberSynced: blockNumber,
		stateDiffCollector:      m,
		startingBlockNumber:     blockNumber,
	}
	s.syncManager.StoreLatestBlockSaved(blockNumber)
	block := &types.Block{
		BlockNumber: uint64(blockNumber),
		BlockHash:   new(felt.Felt),
	}
	s.blockManager.PutBlock(new(felt.Felt), block)

	got := s.Status()
	if got == nil {
		t.Error("unkown error retrieving sync status")
	}

	want := &types.SyncStatus{
		StartingBlockHash:   "0x0000000000000000000000000000000000000000000000000000000000000000",
		StartingBlockNumber: fmt.Sprintf("%x", s.startingBlockNumber),
		CurrentBlockHash:    block.BlockHash.Hex0x(),
		CurrentBlockNumber:  fmt.Sprintf("%x", block.BlockNumber),
		HighestBlockHash:    s.stateDiffCollector.LatestBlock().BlockHash,
		HighestBlockNumber:  fmt.Sprintf("%x", s.stateDiffCollector.LatestBlock().BlockNumber),
	}

	assert.DeepEqual(t, got, want, gocmp.Comparer(func(x *felt.Felt, y *felt.Felt) bool { return x.CmpCompat(y) == 0 }))
}
