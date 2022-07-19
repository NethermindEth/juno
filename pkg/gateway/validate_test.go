package gateway

import (
	"fmt"
	"testing"
)

func TestIsValid(t *testing.T) {
	tests := [...]struct {
		input string
		want  error
	}{
		{
			// p + 1.
			"0x800000000000011000000000000000000000000000000000000000000000002",
			errOutOfRangeFelt,
		},
		{
			// Hexadecimal string formatted using capital letters.
			"0xABCDE",
			nil,
		},
		{
			// Within range.
			"0x12345",
			nil,
		},
		{
			// p.
			"0x800000000000011000000000000000000000000000000000000000000000001",
			errOutOfRangeFelt,
		},
		{
			// Invalid hexadecimal character 'g'.
			"0xabcdefg",
			errInvalidHex,
		},
		{
			// Zero, which is on the lower boundary of the range.
			"0x0",
			nil,
		},
		{
			// Hexadecimal string with leading zeroes.
			"0x00abc",
			nil,
		},
		{
			// Capital letter in the prefix should be flagged as invalid.
			"0Xabc",
			errInvalidHex,
		},
		{
			// Missing "0x" prefix.
			"abc",
			errInvalidHex,
		},
		{
			// Mixture of upper case and lowercase hexadecimal characters.
			"0x00123456789AaBbCcDdeEFf0",
			nil,
		},
		{
			// Decimal number with valid hexadecimal characters.
			"3.1415",
			errInvalidHex,
		},
	}

	for _, test := range tests {
		got := isValid(test.input)
		if got != test.want {
			fmt.Println(got)
			t.Errorf("isValid(%q) = %x, want %x", test.input, got, test.want)
		}
	}
}
