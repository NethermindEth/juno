package compiler_test

import (
	"testing"

	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/stretchr/testify/assert"
)

func TestConcurrencyLimit(t *testing.T) {
	const gb = 1024 // values are in MB
	tests := []struct {
		name                                                        string
		maxConcurrency                                              uint64
		availableMemory, nodeMemoryReserve, maxMemoryPerCompilation uint64
		want                                                        uint64
	}{
		{
			name:                    "memory budget caps below max parallel",
			maxConcurrency:          10,
			availableMemory:         16 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			want:                    3, // (16-4)/4
		},
		{
			name:                    "max parallel caps below memory budget",
			maxConcurrency:          2,
			availableMemory:         64 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			want:                    2,
		},
		{
			name:                    "exactly one compilation fits",
			maxConcurrency:          10,
			availableMemory:         8 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			want:                    1, // (8-4)/4
		},
		{
			name:                    "no room for a single compilation when available equals reserve",
			maxConcurrency:          10,
			availableMemory:         4 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			want:                    0,
		},
		{
			name:                    "no room when available below reserve plus one compilation",
			maxConcurrency:          10,
			availableMemory:         6 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			want:                    0, // (6-4)/4
		},
		{
			name:                    "unbounded per-compilation memory ignores budget",
			maxConcurrency:          7,
			availableMemory:         1 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 0,
			want:                    7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compiler.ConcurrencyLimit(
				tt.maxConcurrency,
				tt.availableMemory,
				tt.nodeMemoryReserve,
				tt.maxMemoryPerCompilation,
			)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAvailableMemoryMB(t *testing.T) {
	assert.NotZero(t, compiler.AvailableMemoryMB())
}
