package uint128

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
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
			description:    "128-bit #1",
			loBits:         0x0,
			hiBits:         0x0,
			expectedBytes:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expectedString: "0x0",
		},
		{
			description:    "128-bit #2",
			loBits:         0x1,
			hiBits:         0x0,
			expectedBytes:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			expectedString: "0x1",
		},
		{
			description:    "128-bit #3",
			loBits:         0xFFFFFFFFFFFFFFFF,
			hiBits:         0xFFFFFFFFFFFFFFFF,
			expectedBytes:  []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			expectedString: "0xffffffffffffffffffffffffffffffff",
		},
		{
			description:    "128-bit #4",
			loBits:         0x0123456789ABCDEF,
			hiBits:         0x123456789ABCDEF0,
			expectedBytes:  []byte{18, 52, 86, 120, 154, 188, 222, 240, 1, 35, 69, 103, 137, 171, 205, 239},
			expectedString: "0x123456789abcdef00123456789abcdef",
		},
		{
			description:    "128-bit #5",
			loBits:         0xF0D3B8A289C7E5B3,
			hiBits:         0x1A4B7E9C2D3F5A6E,
			expectedBytes:  []byte{26, 75, 126, 156, 45, 63, 90, 110, 240, 211, 184, 162, 137, 199, 229, 179},
			expectedString: "0x1a4b7e9c2d3f5a6ef0d3b8a289c7e5b3",
		},
		{
			description:    "128-bit #6",
			loBits:         0x0,
			hiBits:         0x1A4B7E9C2D3F5A6E,
			expectedBytes:  []byte{26, 75, 126, 156, 45, 63, 90, 110, 0, 0, 0, 0, 0, 0, 0, 0},
			expectedString: "0x1a4b7e9c2d3f5a6e0000000000000000",
		},
		{
			description:    "128-bit #7",
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
			wantedErr: false,
		},
		{
			description: "Valid JSON 2",
			jsonInput:   `{"max_price_per_unit": "0x1"}`,
			expected: &Int{
				0: 0x1,
			},
			wantedErr: false,
		},
		{
			description: "Valid JSON 3",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000000"}`,
			expected: &Int{
				1: 0x1,
			},
			wantedErr: false,
		},
		{
			description: "Valid JSON 4",
			jsonInput:   `{"max_price_per_unit": "0x00000000000000010000000000000001"}`,
			expected: &Int{
				0: 0x1,
				1: 0x1,
			},
			wantedErr: false,
		},
		{
			description: "Valid JSON 5",
			jsonInput:   `{"max_price_per_unit": "0x6e58133b38301a6cdfa34ca991c4ba39"}`,
			expected: &Int{
				0: 0xdfa34ca991c4ba39,
				1: 0x6e58133b38301a6c,
			},
			wantedErr: false,
		},
		{
			description: "Valid JSON 6",
			jsonInput:   `{"max_price_per_unit": "0x5af3107a4000"}`,
			expected: &Int{
				0: 0x5af3107a4000,
			},
			wantedErr: false,
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
			jsonInput:   `{"max_price_per_unit": "0xc4c53de93c6f98fa8a91d4f6683b06d2"}`,
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
			description: "String 1",
			textInput:   "0x5af3107a4000",
			expected: &Int{
				0: 0x5af3107a4000,
			},
			wantedErr: false,
		},
		{
			description: "String 2",
			textInput:   "0x5af3107a40#$^#@($H#(HG(WG_00",
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "String 3",
			textInput:   "IAMNOTAHEXSTRING",
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "String 4",
			textInput:   "01234G6789ABCDEF",
			expected:    &Int{},
			wantedErr:   true,
		},
		{
			description: "String 5",
			textInput:   "0x6e58133b38301a6cdfa34ca991c4ba39",
			expected: &Int{
				0: 0xdfa34ca991c4ba39,
				1: 0x6e58133b38301a6c,
			},
			wantedErr: false,
		},
		{
			description: "String 6",
			textInput:   "0x8ac7230489e80000",
			expected: &Int{
				0: 0x8ac7230489e80000,
			},
			wantedErr: false,
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

func TestUint128MultiplyWithFelt(t *testing.T) {
	tests := []struct {
		description  string
		multiplier   *fp.Element
		multiplicand *Int
		product      *fp.Element
		wantedErr    bool
	}{
		{
			description: "Multiplication Test #1",
			multiplier: &fp.Element{
				0: 0xffffffffffcf2c01,
				1: 0xffffffffffffffff,
				2: 0xffffffffffffffff,
				3: 0x7fffffffcc1ec10,
			},
			multiplicand: &Int{
				0: 0x2540be400,
				1: 0x0,
			},
			product: &fp.Element{
				0: 0xff8e502b67300001,
				1: 0xffffffffffffffff,
				2: 0xffffffffffffffff,
				3: 0x7352e1da300010,
			},
		},
		{
			description: "Multiplication Test #1",
			multiplier: &fp.Element{
				0: 0x0,
				1: 0x0,
				2: 0x0,
				3: 0x0,
			},
			multiplicand: &Int{
				0: 0xffffffffffffffff,
				1: 0xffffffffffffffff,
			},
			product: &fp.Element{
				0: 0x0,
				1: 0x0,
				2: 0x0,
				3: 0x0,
			},
		},
		{
			description: "Multiplication Test #1",
			multiplier: &fp.Element{
				0: 0xffffffffffffffff,
				1: 0xffffffffffffffff,
				2: 0xffffffffffffffff,
				3: 0xffffffffffffffff,
			},
			multiplicand: &Int{
				0: 0x0,
				1: 0x0,
			},
			product: &fp.Element{
				0: 0x0,
				1: 0x0,
				2: 0x0,
				3: 0x0,
			},
		},
		{
			description: "Multiplication Test #1",
			multiplier: &fp.Element{
				0: 0xffffffffffffffff,
				1: 0xffffffffffffffff,
				2: 0xffffffffffffffff,
				3: 0xffffffffffffffff,
			},
			multiplicand: &Int{
				0: 0xffffffffffffffff,
				1: 0xffffffffffffffff,
			},
			product: &fp.Element{
				0: 0xffffffffff6f8022,
				1: 0x43ff,
				2: 0xffffffffffffffdf,
				3: 0x7fffffff6678230,
			},
			wantedErr: true,
		},
	}
	{
		for _, test := range tests {
			f := felt.NewFelt(test.multiplier)
			actual := new(Int).MulWithFelt(f, test.multiplicand)
			e := felt.NewFelt(test.product)

			expected := new(big.Int)
			expected = e.BigInt(expected)

			if test.wantedErr {
				return
			}

			if (actual.Cmp(e.BigInt(expected))) != 0 {
				if len(actual.Bytes()) != len(expected.Bytes()) {
					// probably an integer overflow that big.Int can interpret but not a felt.Felt
					t.Errorf("actual Bytes() length=%d differs from expected Bytes() length=%d", len(actual.Bytes()), len(expected.Bytes()))
				}
				t.Errorf("multiplication not equal")
			}
		}
	}
}
