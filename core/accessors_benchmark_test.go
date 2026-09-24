package core_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/starknet"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/stretchr/testify/require"
)

// Sampled ~1000 random mainnet blocks (heights 5M-11.5M) on 2026-07-07.
// These 3 are the closest match to that sample's p50/p95/p99 storage-diff count.
var stateUpdateWithBlockFixtures = []struct {
	percentile string
	file       string
}{
	{"p50", "5782224.json"},
	{"p95", "8582459.json"},
	{"p99", "9706496.json"},
}

type quietPebbleLogger struct{}

func (quietPebbleLogger) Infof(string, ...any)  {}
func (quietPebbleLogger) Errorf(string, ...any) {}
func (quietPebbleLogger) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

func newBenchDB(b *testing.B) db.KeyValueStore {
	b.Helper()
	database, err := pebblev2.New(b.TempDir(), pebblev2.WithLogger(quietPebbleLogger{}))
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, database.Close()) })
	return database
}

func loadStateUpdateWithBlockFixture(b *testing.B, file string) (*core.Block, *core.StateUpdate) {
	b.Helper()
	path := filepath.Join("testdata", "accessors", "state_update_with_block", file)
	data, err := os.ReadFile(path)
	require.NoError(b, err)

	var raw starknet.StateUpdateWithBlockAndSignature
	require.NoError(b, json.Unmarshal(data, &raw))

	block, err := sn2core.AdaptBlock(raw.Block, raw.Signature)
	require.NoError(b, err)
	su, err := sn2core.AdaptStateUpdate(raw.StateUpdate)
	require.NoError(b, err)
	return block, su
}

func BenchmarkReadStateUpdateByBlockNum_Mainnet(b *testing.B) {
	for _, f := range stateUpdateWithBlockFixtures {
		b.Run(f.percentile, func(b *testing.B) {
			_, su := loadStateUpdateWithBlockFixture(b, f.file)
			const blockNum = uint64(0)

			database := newBenchDB(b)
			require.NoError(b, core.WriteStateUpdateByBlockNum(database, blockNum, su))

			_, err := core.GetStateUpdateByBlockNum(database, blockNum)
			require.NoError(b, err)

			b.ReportAllocs()
			for b.Loop() {
				if _, err := core.GetStateUpdateByBlockNum(database, blockNum); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkReadBlockByNumber_Mainnet(b *testing.B) {
	for _, f := range stateUpdateWithBlockFixtures {
		b.Run(f.percentile, func(b *testing.B) {
			block, _ := loadStateUpdateWithBlockFixture(b, f.file)
			const blockNum = uint64(0)
			block.Header.Number = blockNum

			database := newBenchDB(b)
			require.NoError(b, core.WriteBlockHeaderByNumber(database, block.Header))
			require.NoError(b, core.WriteTransactionsAndReceipts(
				database, blockNum, block.Transactions, block.Receipts,
			))

			_, err := core.GetBlockByNumber(database, blockNum)
			require.NoError(b, err)

			b.ReportAllocs()
			for b.Loop() {
				if _, err := core.GetBlockByNumber(database, blockNum); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
