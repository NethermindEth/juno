package core

import (
	"encoding/json"
	"testing"
)

func TestUint128Bytes(t *testing.T) {
	tests := []struct {
		description string
		input       *Uint128
		expected    []byte
	}{
		{
			description: "Bytes 1",
			input:       NewUint128(0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF),
			expected:    []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			description: "Bytes 2",
			input:       NewUint128(0x123456789ABCDEF0, 0x0123456789ABCDEF),
			expected:    []byte{18, 52, 86, 120, 154, 188, 222, 240, 1, 35, 69, 103, 137, 171, 205, 239},
		},
		{
			description: "Bytes 3",
			input:       NewUint128(0x1A4B7E9C2D3F5A6E, 0xF0D3B8A289C7E5B3),
			expected:    []byte{26, 75, 126, 156, 45, 63, 90, 110, 240, 211, 184, 162, 137, 199, 229, 179},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := test.input.Bytes()
			if !byteSlicesEqual(result, test.expected) {
				t.Errorf("Expected %x, got %x", test.expected, result)
			}
		})
	}
}

func TestUnmarshalJsonToUint128(t *testing.T) {
	tests := []struct {
		description string
		jsonInput   string
		expected    *Uint128
		wantedErr   bool
	}{
		{
			description: "Valid JSON",
			jsonInput:   `{"max_price_per_unit": "0x0"}`,
			expected:    &Uint128{hi: 0x0, lo: 0x0},
			wantedErr:   false,
		},
		{
			description: "Valid JSON",
			jsonInput:   `{"max_price_per_unit": "0x1"}`,
			expected:    &Uint128{hi: 0x0, lo: 0x1},
			wantedErr:   false,
		},
		{
			description: "Valid JSON",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000000"}`,
			expected:    &Uint128{hi: 0x1, lo: 0x0},
			wantedErr:   false,
		},
		{
			description: "Valid JSON",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000001"}`,
			expected:    &Uint128{hi: 0x1, lo: 0x1},
			wantedErr:   false,
		},
		{
			description: "Valid JSON",
			jsonInput:   `{"max_price_per_unit": "0x6e58133b38301a6cdfa34ca991c4ba39"}`,
			expected:    &Uint128{hi: 0x6e58133b38301a6c, lo: 0xdfa34ca991c4ba39},
			wantedErr:   false,
		},
		{
			description: "Valid JSON",
			jsonInput:   `{"max_price_per_unit": "foobar"}`,
			expected:    &Uint128{hi: 0x6e58133b38301a6c, lo: 0xdfa34ca991c4ba39},
			wantedErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			var data struct {
				Value Uint128 `json:"max_price_per_unit"`
			}

			err := json.Unmarshal([]byte(test.jsonInput), &data)
			if (err != nil) != test.wantedErr {
				t.Errorf("unable to unmarshal json; got %v, wantedErr %v", err, test.wantedErr)
				return
			}

			if test.wantedErr {
				return
			}

			if !(test.expected.Equal(data.Value)) {
				t.Errorf("Got %v, but we expected %v", &data.Value, test.expected)
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
