package uint128

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestUint128Bytes(t *testing.T) {
	tests := []struct {
		description    string
		loBits         uint64
		hiBits         uint64
		expectedBytes  []byte
		expectedString string
	}{
		{
			description:    "zero byte array",
			loBits:         0x0,
			hiBits:         0x0,
			expectedBytes:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expectedString: "0x0",
		},
		{
			description:    "byte array with value 1/0x1",
			loBits:         0x1,
			hiBits:         0x0,
			expectedBytes:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			expectedString: "0x1",
		},
		{
			description:    "max 128-bit byte array",
			loBits:         0xFFFFFFFFFFFFFFFF,
			hiBits:         0xFFFFFFFFFFFFFFFF,
			expectedBytes:  []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			expectedString: "0xffffffffffffffffffffffffffffffff",
		},
		{
			description:    "big 128-bit value",
			loBits:         0x0123456789ABCDEF,
			hiBits:         0x123456789ABCDEF0,
			expectedBytes:  []byte{18, 52, 86, 120, 154, 188, 222, 240, 1, 35, 69, 103, 137, 171, 205, 239},
			expectedString: "0x123456789abcdef00123456789abcdef",
		},
		{
			description:    "another big 128-bit value",
			loBits:         0xF0D3B8A289C7E5B3,
			hiBits:         0x1A4B7E9C2D3F5A6E,
			expectedBytes:  []byte{26, 75, 126, 156, 45, 63, 90, 110, 240, 211, 184, 162, 137, 199, 229, 179},
			expectedString: "0x1a4b7e9c2d3f5a6ef0d3b8a289c7e5b3",
		},
		{
			description:    "upper bits full, lower bits empty",
			loBits:         0x0,
			hiBits:         0x1A4B7E9C2D3F5A6E,
			expectedBytes:  []byte{26, 75, 126, 156, 45, 63, 90, 110, 0, 0, 0, 0, 0, 0, 0, 0},
			expectedString: "0x1a4b7e9c2d3f5a6e0000000000000000",
		},
		{
			description:    "small values in upper and lower bits",
			loBits:         0x1,
			hiBits:         0x1,
			expectedBytes:  []byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1},
			expectedString: "0x10000000000000001",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			arr := make([]uint64, 2)
			arr[0] = test.loBits
			arr[1] = test.hiBits
			actual := Int(arr)
			if test.expectedString != actual.String() {
				t.Errorf("Expected string=%s, got string=%s", test.expectedString, actual.String())
			}
			if !bytes.Equal(actual.Bytes(), test.expectedBytes) {
				t.Errorf("Expected arr=%x, got %x", test.expectedBytes, actual.Bytes())
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
			expected: &Int{
				0: 0x0,
			},
		},
		{
			description: "Valid JSON 2",
			jsonInput:   `{"max_price_per_unit": "0x1"}`,
			expected: &Int{
				0: 0x1,
			},
		},
		{
			description: "Valid JSON 3",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000000"}`,
			expected: &Int{
				1: 0x1,
			},
		},
		{
			description: "Valid JSON 4",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000001"}`,
			expected: &Int{
				0: 0x1,
				1: 0x1,
			},
		},
		{
			description: "Valid JSON 5",
			jsonInput:   `{"max_price_per_unit": "0x6e58133b38301a6cdfa34ca991c4ba39"}`,
			expected: &Int{
				0: 0xdfa34ca991c4ba39,
				1: 0x6e58133b38301a6c,
			},
		},
		{
			description: "Valid JSON 6",
			jsonInput:   `{"max_price_per_unit": "0x5af3107a4000"}`,
			expected: &Int{
				0: 0x5af3107a4000,
			},
		},
		{
			description: "Invalid JSON 1",
			jsonInput:   `{"max_price_per_unit": "foobar"}`,
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "Invalid JSON 2",
			jsonInput:   `{"max_price_per_unit": "q2i34jti0q2ngioawngioasnjgoanrjognwoignwejogniaewognkoaergnoarnggkangionw34gion3"}`,
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "Invalid JSON 3",
			jsonInput:   `{"max_price_per_unit": ""}`,
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "Invalid JSON 4",
			jsonInput:   `{"max_price_per_unit": "0xc4c53de93c6fffffffff98fa8a91d4f6683b06d2"}`,
			expected:    &Int{},
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
				if test.wantedErr {
					return
				}
				t.Errorf("unable to unmarshal json; gotError %v, wantedErr %v", err, test.wantedErr)
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
		expected    *Int
		wantedErr   bool
	}{
		{
			description: "invalid hex characters",
			textInput:   "0x5af3107a40#$^#@($H#(HG(WG_00",
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "valid all bits filled hexadecimal",
			textInput:   "0x6e58133b38301a6cdfa34ca991c4ba39",
			expected: &Int{
				0: 0xdfa34ca991c4ba39,
				1: 0x6e58133b38301a6c,
			},
		},
		{
			description: "valid least significant bits hexadecimal",
			textInput:   "0x5af3107a4000",
			expected: &Int{
				0: 0x5af3107a4000,
			},
		},
		{
			description: "valid most significant bits hexadecimal",
			textInput:   "0xf0000000000000000",
			expected: &Int{
				1: 0xf,
			},
		},
	}
	{
		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				i := &Int{}
				actual, err := i.SetString(test.textInput)
				if err != nil {
					if test.wantedErr {
						return
					}
					t.Errorf("failed to set string %s on &Int{}", test.textInput)
				}
				if !actual.Equal(test.expected) {
					t.Errorf("got %v, expected=%v", actual, test.expected)
				}
			})
		}
	}
}
