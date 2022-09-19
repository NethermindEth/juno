package validate

import (
	"fmt"
	"strconv"
	"testing"

	"gotest.tools/assert"
)

func TestFelt(t *testing.T) {
	tests := [...]struct {
		input string
		// want represents whether a case is valid.
		want bool
	}{
		{
			// p + 1.
			"0x0800000000000011000000000000000000000000000000000000000000000002",
			false,
		},
		{
			// Hexadecimal string formatted using capital letters.
			"0x0ABCDE",
			true,
		},
		{
			// Within range.
			"0x012345",
			true,
		},
		{
			// p.
			"0x0800000000000011000000000000000000000000000000000000000000000001",
			false,
		},
		{
			// Invalid hexadecimal character 'g'.
			"0xabcdefg",
			false,
		},
		{
			// Zero, which is on the lower boundary of the range.
			"0x00",
			true,
		},
		{
			// Hexadecimal string with leading zeroes.
			"0x00abc",
			true,
		},
		{
			// Capital letter in the prefix should be flagged as invalid.
			"0Xabc",
			false,
		},
		{
			// Missing "0x" prefix.
			"abc",
			false,
		},
		{
			// Valid hexadecimal string but the StarkNet JSON-RPC
			// specification requires that there is at least one leading 0.
			"0xabc",
			false,
		},
		{
			// Valid hexadecimal string but the StarkNet JSON-RPC
			// specification requires that there is at least one leading 0
			// (and at least one other character 😅).
			"0x0",
			false,
		},
		{
			// Mixture of upper case and lowercase hexadecimal characters.
			"0x00123456789AaBbCcDdeEFf0",
			true,
		},
		{
			// Decimal number with valid hexadecimal characters.
			"3.1415",
			false,
		},
	}

	for _, test := range tests {
		t.Run("validate.Felt("+strconv.Quote(test.input)+")", func(t *testing.T) {
			got := Felt(test.input)
			assert.Check(t, got == test.want, "Felt(%q) = %t, want %t", test.input, got, test.want)
		})
	}
}

func TestFelts(t *testing.T) {
	tests := [...]struct {
		input []string
		want  bool
	}{
		{
			[]string{""},
			false,
		},
		{
			[]string{"0x0abc" /* valid */, "0x0" /* not valid */},
			false,
		},
		{
			[]string{"0xabc " /* not valid */, "0x01" /* valid */},
			false,
		},
		{
			[]string{"0x0abc", "0x00"},
			true,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("validate.Felts(%#v)", test.input), func(t *testing.T) {
			got := Felts(test.input)
			assert.Check(t, got == test.want, "Felts(%#v) = %t, want %t", test.input, got, test.want)
		})
	}
}
