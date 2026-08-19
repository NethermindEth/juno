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
		expected                                                    uint64
	}{
		{
			name:                    "memory budget caps below max parallel",
			maxConcurrency:          10,
			availableMemory:         16 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			expected:                3, // (16-4)/4
		},
		{
			name:                    "max parallel caps below memory budget",
			maxConcurrency:          2,
			availableMemory:         64 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			expected:                2,
		},
		{
			name:                    "exactly one compilation fits",
			maxConcurrency:          10,
			availableMemory:         8 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			expected:                1, // (8-4)/4
		},
		{
			name:                    "floors to 1 when available equals reserve",
			maxConcurrency:          10,
			availableMemory:         4 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			expected:                1,
		},
		{
			name:                    "floors to 1 when memory fits no compilation",
			maxConcurrency:          10,
			availableMemory:         6 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 4 * gb,
			expected:                1, // (6-4)/4 = 0, floored to 1
		},
		{
			name:                    "unbounded per-compilation memory ignores budget",
			maxConcurrency:          7,
			availableMemory:         1 * gb,
			nodeMemoryReserve:       4 * gb,
			maxMemoryPerCompilation: 0,
			expected:                7,
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
			assert.Equal(t, tt.expected, got)
		})
	}
}
