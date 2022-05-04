package starknet

import "testing"

func TestClean(t *testing.T) {
	tests := [...]struct {
		entry    string
		expected string
	}{
		{
			"0x001111",
			"1111",
		},
		{
			"04a1111",
			"4a1111",
		},
		{
			"000000001",
			"1",
		},
		{
			"000ssldkfmsd1111",
			"ssldkfmsd1111",
		},
		{
			"000111sp",
			"111sp",
		},
	}

	for _, test := range tests {
		answer := clean(test.entry)
		if answer != test.expected {
			t.Fail()
		}
	}
}
