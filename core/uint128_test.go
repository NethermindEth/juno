package core

import "testing"

func TestUint128Bytes(t *testing.T) {
	tests := []struct {
		name     string
		input    *Uint128
		expected []byte
	}{
		{"Bytes 1", NewUint128(0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF), []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}},
		{"Bytes 2", NewUint128(0x123456789ABCDEF0, 0x0123456789ABCDEF), []byte{18, 52, 86, 120, 154, 188, 222, 240, 1, 35, 69, 103, 137, 171, 205, 239}},
		{"Bytes 3", NewUint128(0x1A4B7E9C2D3F5A6E, 0xF0D3B8A289C7E5B3), []byte{26, 75, 126, 156, 45, 63, 90, 110, 240, 211, 184, 162, 137, 199, 229, 179}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.input.Bytes()
			if !byteSlicesEqual(result, test.expected) {
				t.Errorf("Expected %x, got %x", test.expected, result)
			}
		})
	}
}

func byteSlicesEqual(b1 []byte, b2 []byte) bool {
	if len(b1) != len(b2) {
		return false
	}
	for i := range b1 {
		if b1[i] != b2[i] {
			return false
		}
	}
	return true
}
