package common

import "testing"

func TestIsHex(t *testing.T) {
	tests := [...]struct {
		Input string
		Want  bool
	}{
		{
			"0x7932de7ec535bfd45e2951a35c06e13d22188cb7eb7b7cc43454ee63df78aff", true,
		},
		{
			"0x8fccde9ae0ca4da714b8f27d4aaf6edfa8ce92a4", true,
		},
		{
			"0x0", true,
		},
		{
			"0xzxc", false,
		},
	}
	for _, test := range tests {
		out := IsHex(test.Input)
		if out != test.Want {
			t.Errorf("IsHex(%s) = %v, want: %v", test.Input, out, test.Want)
		}
	}
}
