package migration_test

import (
	"testing"

	"github.com/NethermindEth/juno/migration"
	"github.com/stretchr/testify/require"
)

func TestSchemaVersion_Set(t *testing.T) {
	tests := []struct {
		name     string
		indices  []uint8
		expected migration.SchemaVersion
	}{
		{
			name:     "set single bit",
			indices:  []uint8{0},
			expected: 1, // 0b0001
		},
		{
			name:     "set multiple bits",
			indices:  []uint8{0, 2, 5},
			expected: 0b100101, // bits 0, 2, 5
		},
		{
			name:     "set bit 63 (max)",
			indices:  []uint8{63},
			expected: 1 << 63,
		},
		{
			name:     "idempotent - set same bit twice",
			indices:  []uint8{3, 3},
			expected: 8, // 0b1000 (bit 3)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sv migration.SchemaVersion
			for _, idx := range tt.indices {
				sv.Set(idx)
			}

			if sv != tt.expected {
				t.Errorf("got %b, want %b", sv, tt.expected)
			}
		})
	}
}

func TestSchemaVersion_Has(t *testing.T) {
	var sv migration.SchemaVersion
	sv.Set(0)
	sv.Set(5)
	sv.Set(10)
	sv.Set(63)

	tests := []struct {
		name     string
		index    uint8
		expected bool
	}{
		{"bit 0 set", 0, true},
		{"bit 5 set", 5, true},
		{"bit 10 set", 10, true},
		{"bit 63 set", 63, true},
		{"bit 1 not set", 1, false},
		{"bit 62 not set", 62, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(
				t, tt.expected, sv.Has(tt.index),
				"Has(%d) = %v, want %v", tt.index, sv.Has(tt.index), tt.expected,
			)
		})
	}
}

func TestSchemaVersion_Len(t *testing.T) {
	tests := []struct {
		name     string
		sv       migration.SchemaVersion
		expected int
	}{
		{"empty", 0, 0},
		{"bit 0 only", 1, 1},
		{"bit 63 only", 1 << 63, 1},
		{"two bits", 0b101, 2},
		{"bits 0 and 63", 1 | (1 << 63), 2},
		{"all bits", ^migration.SchemaVersion(0), 64},
		{"bits 0,2,5", 0b100101, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(
				t, tt.expected, tt.sv.Len(),
				"Len() = %d, want %d", tt.sv.Len(), tt.expected,
			)
		})
	}
}

func TestSchemaVersion_Iter(t *testing.T) {
	tests := []struct {
		name     string
		sv       migration.SchemaVersion
		expected []uint8
	}{
		{"empty", 0, []uint8{}},
		{"single bit", 1, []uint8{0}},
		{"bits 0,2,5", 0b100101, []uint8{0, 2, 5}},
		{"bits 1,3,7", 0b10001010, []uint8{1, 3, 7}},
		{"bit 63 only", 1 << 63, []uint8{63}},
		{"all bits", ^migration.SchemaVersion(0), func() []uint8 {
			result := make([]uint8, 64)
			for i := range 64 {
				result[i] = uint8(i)
			}
			return result
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0
			expectedLen := len(tt.expected)
			for idx := range tt.sv.Iter() {
				require.Less(t, i, expectedLen, "Iter() produced more indices than expected")
				require.Equal(t, tt.expected[i], idx)
				i++
			}
			require.Equal(t, expectedLen, i, "Iter() produced fewer indices than expected")
		})
	}
}

func TestSchemaVersion_Contains(t *testing.T) {
	tests := []struct {
		name     string
		sv       migration.SchemaVersion
		other    migration.SchemaVersion
		expected bool
	}{
		{"empty contains empty", 0, 0, true},
		{"non-empty contains empty", 0b101, 0, true},
		{"empty does not contain non-empty", 0, 0b101, false},
		{"exact match", 0b101, 0b101, true},
		{"contains all", 0b1111, 0b1010, true},
		{"contains all and more", 0b1111, 0b0101, true},
		{"does not contain missing bit", 0b0010, 0b0110, false},
		{"does not contain missing multiple", 0b0001, 0b1111, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.sv.Contains(tt.other))
		})
	}
}

func TestSchemaVersion_Union(t *testing.T) {
	tests := []struct {
		name     string
		sv       migration.SchemaVersion
		other    migration.SchemaVersion
		expected migration.SchemaVersion
	}{
		{
			name:     "union with empty",
			sv:       0b1010,
			other:    0,
			expected: 0b1010,
		},
		{
			name:     "union empty with non-empty",
			sv:       0,
			other:    0b1010,
			expected: 0b1010,
		},
		{
			name:     "union non-overlapping bits",
			sv:       0b0010, // bit 1
			other:    0b1000, // bit 3
			expected: 0b1010, // bits 1 and 3
		},
		{
			name:     "union overlapping bits",
			sv:       0b1010, // bits 1, 3
			other:    0b1100, // bits 2, 3
			expected: 0b1110, // bits 1, 2, 3
		},
		{
			name:     "union with all bits",
			sv:       0b0101,
			other:    ^migration.SchemaVersion(0),
			expected: ^migration.SchemaVersion(0),
		},
		{
			name:     "union same values",
			sv:       0b1010,
			other:    0b1010,
			expected: 0b1010,
		},
		{
			name:     "union multiple bits",
			sv:       0b0001, // bit 0
			other:    0b1110, // bits 1, 2, 3
			expected: 0b1111, // bits 0, 1, 2, 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.sv.Union(tt.other))
		})
	}
}

func TestSchemaVersion_HighestBit(t *testing.T) {
	tests := []struct {
		name     string
		sv       migration.SchemaVersion
		expected int
	}{
		{"empty", 0, -1},
		{"single bit", 1, 0},
		{"bits 0,2,5", 0b100101, 5},
		{"bits 1,3,7", 0b10001010, 7},
		{"bit 63 only", 1 << 63, 63},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.sv.HighestBit())
		})
	}
}

func TestSchemaVersion_Difference(t *testing.T) {
	tests := []struct {
		name     string
		sv       migration.SchemaVersion
		other    migration.SchemaVersion
		expected migration.SchemaVersion
	}{
		{"empty minus empty", 0, 0, 0},
		{"bit 0 minus empty", 1, 0, 1},
		{"bits 0,2,5 minus bits 3,5", 0b100101, 0b101000, 0b000101},
		{"bits 1,3,7 minus bits 1,7", 0b10001010, 0b10000010, 0b00001000},
		{"bit 63 minus empty", 1 << 63, 0, 1 << 63},
		{"bits 1,3 minus all bits", 0b1010, ^migration.SchemaVersion(0), 0},
		{"bits 1,3 minus same bits", 0b1010, 0b1010, 0},
		{"bit 0 minus bits 1,2,3", 0b0001, 0b1110, 0b0001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.sv.Difference(tt.other))
		})
	}
}
