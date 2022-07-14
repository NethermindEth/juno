package gateway

import "testing"
import "fmt"

func TestIsValid(t *testing.T) {
	tests := [...]struct {
		input string
		want  error
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
		got := isValid(test.input)
		if got != test.want {
			fmt.Println(got)
			t.Errorf("isValid(%x) = %x, want %x", test.input, got, test.want)
		}
	}
}
