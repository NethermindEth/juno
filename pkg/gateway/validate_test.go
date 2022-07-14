package gateway

import "testing"

func TestIsValid(t *testing.T) {
	tests := [...]struct {
		entry    string
		expected error
	}{
		{
			"0x800000000000011000000000000000000000000000000000000000000000002",
			errOutOfRangeFelt,
		},
		{
			"0xA000000000000022000000000000000000000000000000000000000000000001",
			errOutOfRangeFelt,
		},
		{
			"0x800000000000011000000000000000000000000000000000000000000000000",
			nil,
		},
		{
			"0x800000000000011000000000000000000000000000000000000000000000001",
			errOutOfRangeFelt,
		},
		{
			"0x80000000000001100000000000000000000000000000000000000000000000t",
			errInvalidHex,
		},
		{
			"0x0",
			nil,
		},
	}
	for _, test := range tests {
		result := isValid(test.entry)
		if result != test.expected {
			t.Errorf("isValid() = %x, want %x", result, test.expected)
		}
	}
}
