package uint128

import (
	"encoding/json"
	"testing"
)

func TestUint128Bytes(t *testing.T) {
	tests := []struct {
		description  string
		input        *Int
		expected_arr []byte
		expected_str string
	}{
		{
			description:  "128-bit #1",
			input:        NewInt128(0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF),
			expected_arr: []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			expected_str: "ffffffffffffffffffffffffffffffff",
		},
		{
			description:  "128-bit #2",
			input:        NewInt128(0x123456789ABCDEF0, 0x0123456789ABCDEF),
			expected_arr: []byte{18, 52, 86, 120, 154, 188, 222, 240, 1, 35, 69, 103, 137, 171, 205, 239},
			expected_str: "123456789abcdef00123456789abcdef",
		},
		{
			description:  "128-bit 3",
			input:        NewInt128(0x1A4B7E9C2D3F5A6E, 0xF0D3B8A289C7E5B3),
			expected_arr: []byte{26, 75, 126, 156, 45, 63, 90, 110, 240, 211, 184, 162, 137, 199, 229, 179},
			expected_str: "1a4b7e9c2d3f5a6ef0d3b8a289c7e5b3",
		},
		{
			description:  "128-bit 4",
			input:        NewInt128(0x1A4B7E9C2D3F5A6E, 0x0),
			expected_arr: []byte{26, 75, 126, 156, 45, 63, 90, 110, 0, 0, 0, 0, 0, 0, 0, 0},
			expected_str: "1a4b7e9c2d3f5a6e0000000000000000",
		},
		{
			description:  "128-bit 5",
			input:        NewInt128(0x0, 0x0),
			expected_arr: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected_str: "00000000000000000000000000000000",
		},
		{
			description:  "128-bit 6",
			input:        NewInt128(0x0, 0x1),
			expected_arr: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			expected_str: "00000000000000000000000000000001",
		},
		{
			description:  "128-bit 7",
			input:        NewInt128(0x1, 0x0),
			expected_arr: []byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0},
			expected_str: "00000000000000010000000000000000",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if test.expected_str != test.input.String() {
				t.Errorf("Expected string=%s, got string=%s", test.expected_str, test.input.String())
			}
			result := test.input.Bytes()
			if !byteSlicesEqual(result, test.expected_arr) {
				t.Errorf("Expected arr=%x, got %x", test.expected_arr, result)
			}
		})
	}
}

func TestUint64Bytes(t *testing.T) {
	tests := []struct {
		description  string
		input        *Int
		expected_arr []byte
		expected_str string
	}{
		{
			description:  "64-bit #1",
			input:        NewInt64(0xFFFFFFFFFFFFFFFF),
			expected_arr: []byte{0, 0, 0, 0, 0, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255},
			expected_str: "0000000000000000ffffffffffffffff",
		},
		{
			description:  "64-bit #2",
			input:        NewInt64(0x0123456789ABCDEF),
			expected_arr: []byte{0, 0, 0, 0, 0, 0, 0, 0, 1, 35, 69, 103, 137, 171, 205, 239},
			expected_str: "00000000000000000123456789abcdef",
		},
		{
			description:  "64-bit 3",
			input:        NewInt64(0x0000000000000000),
			expected_arr: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected_str: "00000000000000000000000000000000",
		},
		{
			description:  "64-bit 4",
			input:        NewInt128(0x1A4B7E9C2D3F5A6E, 0x0),
			expected_arr: []byte{26, 75, 126, 156, 45, 63, 90, 110, 0, 0, 0, 0, 0, 0, 0, 0},
			expected_str: "1a4b7e9c2d3f5a6e0000000000000000",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if test.expected_str != test.input.String() {
				t.Errorf("Expected string=%s, got string=%s", test.expected_str, test.input.String())
			}
			result := test.input.Bytes()
			if !byteSlicesEqual(result, test.expected_arr) {
				t.Errorf("Expected arr=%x, got %x", test.expected_arr, result)
			}
		})
	}
}

func TestUnmarshalJsonToUint128(t *testing.T) {
	tests := []struct {
		description string
		jsonInput   string
		expected    *Int
		wantedErr   bool
	}{
		{
			description: "Valid JSON 1",
			jsonInput:   `{"max_price_per_unit": "0x0"}`,
			expected:    NewInt64(0),
			wantedErr:   false,
		},
		{
			description: "Valid JSON 2",
			jsonInput:   `{"max_price_per_unit": "0x1"}`,
			expected:    NewInt64(1),
			wantedErr:   false,
		},
		{
			description: "Valid JSON 3",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000000"}`,
			expected:    NewInt128(0x1, 0x0),
			wantedErr:   false,
		},
		{
			description: "Valid JSON 4",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000001"}`,
			expected:    NewInt128(0x1, 0x1),
			wantedErr:   false,
		},
		{
			description: "Valid JSON 5",
			jsonInput:   `{"max_price_per_unit": "0x6e58133b38301a6cdfa34ca991c4ba39"}`,
			expected:    NewInt128(0x6e58133b38301a6c, 0xdfa34ca991c4ba39),
			wantedErr:   false,
		},
		{
			description: "Valid JSON 6",
			jsonInput:   `{"max_price_per_unit": "foobar"}`,
			expected:    NewInt128(0x6e58133b38301a6c, 0xdfa34ca991c4ba39),
			wantedErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			var data struct {
				Value *Int `json:"max_price_per_unit"`
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
				t.Errorf("got %v, but we expected %v", &data.Value, test.expected)
			}
		})
	}
}

func TestUint128FromHexString(t *testing.T) {
	tests := []struct {
		description string
		textInput   string
		expected64  *Int
		expected128 *Int
	}{
		{
			description: "String 1",
			textInput:   "0x5af3107a4000",
			expected64:  NewInt64(0x5af3107a4000),
			expected128: NewInt128(0x0, 0x5af3107a4000),
		},
	}
	{
		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				result := NewUint128(test.textInput)
				if !result.Equal(test.expected64) && !result.Equal(test.expected128) {
					t.Errorf("got %v, expected64=%v, expected128=%v", result, test.expected64, test.expected128)
				}
			})
		}
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
