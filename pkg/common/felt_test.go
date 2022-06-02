package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
)

func TestBigToFelt(t *testing.T) {
	type TestCase struct {
		Input *big.Int
		Want  Felt
	}
	newTestCase := func(hexValue string, want Felt) TestCase {
		input, _ := new(big.Int).SetString(hexValue[2:], 16)
		return TestCase{input, want}
	}
	tests := [...]TestCase{
		newTestCase(
			"0xa",
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
		),
		newTestCase(
			"0x2ca7c49c9b7f58265a",
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90},
		),
		newTestCase(
			"0x9cb17c8249aa59d4561deb4d1e51d640c49f355bf74c76a6c3",
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195},
		),
		newTestCase(
			"0xc1e3718ac229397d192530c0ca37982fd6609a2b9efa2fbc09c1df983f43d225",
			[FeltLength]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37},
		),
	}
	for _, test := range tests {
		f := BigToFelt(test.Input)
		if bytes.Compare(f[:], test.Want[:]) != 0 {
			t.Errorf("BigToFelt(%s) = %s, want %s", test.Input.Text(16), f, test.Want)
		}
	}
}

func TestHexToFelt(t *testing.T) {
	type TestCase struct {
		Input string
		Want  Felt
	}
	tests := [...]TestCase{
		{
			"0xa",
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
		},
		{
			"0x2ca7c49c9b7f58265a",
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90},
		},
		{
			"0x9cb17c8249aa59d4561deb4d1e51d640c49f355bf74c76a6c3",
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195},
		},
		{
			"0xc1e3718ac229397d192530c0ca37982fd6609a2b9efa2fbc09c1df983f43d225",
			[FeltLength]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37},
		},
	}
	for _, test := range tests {
		f := HexToFelt(test.Input)
		if bytes.Compare(f[:], test.Want[:]) != 0 {
			t.Errorf("HexToFelt(%s) = %s, want %s", test.Input, f, test.Want)
		}
	}
}

func TestFelt_Bytes(t *testing.T) {
	type TestCase struct {
		Input Felt
		Want  []byte
	}
	newTestCase := func(b []byte) TestCase {
		var f Felt
		copy(f[:], b[:32])
		return TestCase{
			Input: f,
			Want:  b,
		}
	}
	tests := [...]TestCase{
		newTestCase([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10}),
		newTestCase([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90}),
		newTestCase([]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195}),
		newTestCase([]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37}),
	}
	for _, test := range tests {
		b := test.Input.Bytes()
		if bytes.Compare(b, test.Want) != 0 {
			t.Errorf("%s.Bytes() = %v, want %v", test.Input, b, test.Want)
		}
	}
}

func TestFelt_Big(t *testing.T) {
	type TestCase struct {
		Input Felt
		Want  *big.Int
	}
	tests := [...]TestCase{
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
			new(big.Int).SetBytes([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10}),
		},
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90},
			new(big.Int).SetBytes([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90}),
		},
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195},
			new(big.Int).SetBytes([]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195}),
		},
		{
			[FeltLength]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37},
			new(big.Int).SetBytes([]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37}),
		},
	}
	for _, test := range tests {
		b := test.Input.Big()
		if b.Cmp(test.Want) != 0 {
			t.Errorf("%s.Bug() = %s, want %s", test.Input, b.Text(16), test.Want.Text(16))
		}
	}
}

func TestFelt_Hex(t *testing.T) {
	type TestCase struct {
		Input Felt
		Want  string
	}
	tests := [...]TestCase{
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
			"0xa",
		},
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90},
			"0x2ca7c49c9b7f58265a",
		},
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195},
			"0x9cb17c8249aa59d4561deb4d1e51d640c49f355bf74c76a6c3",
		},
		{
			[FeltLength]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37},
			"0xc1e3718ac229397d192530c0ca37982fd6609a2b9efa2fbc09c1df983f43d225",
		},
	}
	for _, test := range tests {
		h := test.Input.Hex()
		if h != test.Want {
			t.Errorf("%s.Hex() = %s, want %s", test.Input, h, test.Want)
		}
	}
}

func TestFelt_String(t *testing.T) {
	type TestCase struct {
		Input Felt
		Want  string
	}
	tests := [...]TestCase{
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
			"0xa",
		},
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90},
			"0x2ca7c49c9b7f58265a",
		},
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195},
			"0x9cb17c8249aa59d4561deb4d1e51d640c49f355bf74c76a6c3",
		},
		{
			[FeltLength]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37},
			"0xc1e3718ac229397d192530c0ca37982fd6609a2b9efa2fbc09c1df983f43d225",
		},
	}
	for _, test := range tests {
		h := test.Input.String()
		if h != test.Want {
			t.Errorf("%s.Hex() = %s, want %s", test.Input, h, test.Want)
		}
	}
}

func TestFelt_SetBytes(t *testing.T) {
	type TestCase struct {
		Input []byte
		Want  Felt
	}
	newTestCase := func(b []byte) TestCase {
		var f Felt
		copy(f[:], b[len(b)-32:])
		return TestCase{
			Input: b,
			Want:  f,
		}
	}
	tests := [...]TestCase{
		newTestCase([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10}),
		newTestCase([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90}),
		newTestCase([]byte{0, 0, 0, 0, 0, 0, 0, 156, 177, 124, 130, 73, 170, 89, 212, 86, 29, 235, 77, 30, 81, 214, 64, 196, 159, 53, 91, 247, 76, 118, 166, 195}),
		newTestCase([]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37}),
		newTestCase([]byte{123, 193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37}),
	}
	for _, test := range tests {
		var f Felt
		f.SetBytes(test.Input)
		if bytes.Compare(f[:], test.Want[:]) != 0 {
			t.Errorf("SetBytes(%v) = %v, want %v", test.Input, f, test.Want)
		}
	}
}

func TestFelt_MarshalJSON(t *testing.T) {
	type TestCase struct {
		Input Felt
		Want  []byte
	}
	var tests = [...]TestCase{
		{
			[FeltLength]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
			[]byte("\"0xa\""),
		},
		{
			[FeltLength]byte{193, 227, 113, 138, 194, 41, 57, 125, 25, 37, 48, 192, 202, 55, 152, 47, 214, 96, 154, 43, 158, 250, 47, 188, 9, 193, 223, 152, 63, 67, 210, 37},
			[]byte("\"0xc1e3718ac229397d192530c0ca37982fd6609a2b9efa2fbc09c1df983f43d225\""),
		},
	}
	for _, test := range tests {
		data, err := json.Marshal(test.Input)
		if err != nil {
			t.Error(fmt.Errorf("json.Marshal(%s): %w", test.Input, err))
		}
		if bytes.Compare(data, test.Want) != 0 {
			t.Errorf("json.Marshal(%s) = %s, want: %s", test.Input, data, test.Want)
		}
	}
}
