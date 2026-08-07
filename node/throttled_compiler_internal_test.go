package node

import (
	"testing"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
)

func TestCalculateCompilerConcurrencyBudget(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		cores     uint64
		memMB     uint64
		wantConc  uint64
		wantQueue uint64
	}{
		{
			name:      "db cache is reserved on top of node reserve",
			cfg:       Config{MaxCompilationMemory: 4096, NodeMemoryReserve: 4096, DBCacheSize: 8192},
			cores:     64,
			memMB:     65536,
			wantConc:  13, // (65536 - 4096 - 8192) / 4096
			wantQueue: 26,
		},
		{
			name: "remote db ignores the local cache size",
			cfg: Config{
				MaxCompilationMemory: 4096, NodeMemoryReserve: 4096, DBCacheSize: 8192,
				RemoteDB: "localhost:9090",
			},
			cores:     64,
			memMB:     65536,
			wantConc:  15, // (65536 - 4096) / 4096
			wantQueue: 30,
		},
		{
			name:      "reserve plus cache covering all memory floors to 1",
			cfg:       Config{MaxCompilationMemory: 4096, NodeMemoryReserve: 4096, DBCacheSize: 8192},
			cores:     64,
			memMB:     12288,
			wantConc:  1,
			wantQueue: 2,
		},
		{
			name:      "no compilation memory limit uses core count",
			cfg:       Config{MaxCompilationMemory: 0, NodeMemoryReserve: 4096, DBCacheSize: 8192},
			cores:     64,
			memMB:     65536,
			wantConc:  64,
			wantQueue: 128,
		},
		{
			name: "explicit setting bypasses derivation",
			cfg: Config{
				MaxConcurrentCompilations: 5, MaxConcurrentCompilationsExplicit: true,
				MaxCompilationMemory: 4096, NodeMemoryReserve: 4096, DBCacheSize: 8192,
			},
			cores:     64,
			memMB:     65536,
			wantConc:  5,
			wantQueue: 10,
		},
		{
			name: "explicit queue bypasses derivation",
			cfg: Config{
				MaxCompilationMemory: 4096, NodeMemoryReserve: 4096, DBCacheSize: 8192,
				MaxCompilationQueue: 7, MaxCompilationQueueExplicit: true,
			},
			cores:     64,
			memMB:     65536,
			wantConc:  13,
			wantQueue: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conc, queue := calculateCompilerConcurrencyBudget(
				&tt.cfg, tt.cores, tt.memMB, log.NewNopZapLogger(),
			)
			assert.Equal(t, tt.wantConc, conc)
			assert.Equal(t, tt.wantQueue, queue)
		})
	}
}
