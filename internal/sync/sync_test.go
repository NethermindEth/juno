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

func TestTransactionConverter(t *testing.T) {
	tests := []struct {
		name  string
		input *feeder.TransactionInfo
		want  types.IsTransaction
	}{
		{
			name: "transaction invoke V0",
			input: &feeder.TransactionInfo{
				Transaction: feeder.TxnSpecificInfo{
					TransactionHash:    "0x6525d9aa309e5c80abbdafcc434d53202e06866597cd6dbbc91e5894fad7155",
					ContractAddress:    "0x2fb7ff5b1b474e8e691f5bebad9aa7aa3009f6ef22ccc2816f96cdfe217604d",
					Version:            "0x0",
					EntryPointSelector: "0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6",
					EntryPointType:     "EXTERNAL",
					Calldata:           []string{"0xe3402af6cc1bca3f22d738ab935a5dd8ad1fb230"},
					Signature:          []string{},
					MaxFee:             "0x0",
					Type:               "INVOKE_FUNCTION",
				},
			},
			want: &types.TransactionInvokeV0{
				Hash:               new(felt.Felt).SetHex("0x6525d9aa309e5c80abbdafcc434d53202e06866597cd6dbbc91e5894fad7155"),
				ContractAddress:    new(felt.Felt).SetHex("0x2fb7ff5b1b474e8e691f5bebad9aa7aa3009f6ef22ccc2816f96cdfe217604d"),
				EntryPointSelector: new(felt.Felt).SetHex("0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6"),
				CallData:           []*felt.Felt{new(felt.Felt).SetHex("0xe3402af6cc1bca3f22d738ab935a5dd8ad1fb230")},
				Signature:          []*felt.Felt{},
				MaxFee:             new(felt.Felt).SetHex("0x0"),
			},
		},
		{
			name: "transaction invoke V1",
			input: &feeder.TransactionInfo{
				Transaction: feeder.TxnSpecificInfo{
					TransactionHash:    "0x44dc8c3e13c29fb71cf5d7356ede2d175360ec17d282bb7d12959cf034f7fb9",
					SenderAddress:      "0x5fb7f82414f88e8418bb5f973bbc8fcb660a91913da262f47ecf8e898b83b09",
					Version:            "0x1",
					EntryPointSelector: "0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6",
					EntryPointType:     "EXTERNAL",
					Calldata: []string{
						"0x1",
						"0x79bfdfb3397f4b369e98c3114d0ce71f7fc2283205f408c578fcdf8e827d458",
						"0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6",
						"0x0",
						"0x1",
						"0x1",
						"0xa3bc7d5b2afe2999c817dfbcab22bad20ed7c895",
					},
					Signature: []string{
						"0x72be86958215ae0976704139f29fba3bc450c71883d2d6d93c9256c9160302c",
						"0x741560dbb0527c2c3ac5bd0cff1142d07b55fa2096d2aa158f9ba3ffd06ef85",
					},
					MaxFee: "0x2386f26fc10000",
					Type:   "INVOKE_FUNCTION",
					Nonce:  "0x1",
				},
			},
			want: &types.TransactionInvokeV1{
				Hash:          new(felt.Felt).SetHex("0x44dc8c3e13c29fb71cf5d7356ede2d175360ec17d282bb7d12959cf034f7fb9"),
				SenderAddress: new(felt.Felt).SetHex("0x5fb7f82414f88e8418bb5f973bbc8fcb660a91913da262f47ecf8e898b83b09"),
				CallData: []*felt.Felt{
					new(felt.Felt).SetHex("0x1"),
					new(felt.Felt).SetHex("0x79bfdfb3397f4b369e98c3114d0ce71f7fc2283205f408c578fcdf8e827d458"),
					new(felt.Felt).SetHex("0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6"),
					new(felt.Felt).SetHex("0x0"),
					new(felt.Felt).SetHex("0x1"),
					new(felt.Felt).SetHex("0x1"),
					new(felt.Felt).SetHex("0xa3bc7d5b2afe2999c817dfbcab22bad20ed7c895"),
				},
				Signature: []*felt.Felt{
					new(felt.Felt).SetHex("0x72be86958215ae0976704139f29fba3bc450c71883d2d6d93c9256c9160302c"),
					new(felt.Felt).SetHex("0x741560dbb0527c2c3ac5bd0cff1142d07b55fa2096d2aa158f9ba3ffd06ef85"),
				},
				MaxFee: new(felt.Felt).SetHex("0x2386f26fc10000"),
				Nonce:  new(felt.Felt).SetHex("0x1"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := feederTransactionToDBTransaction(tt.input)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
