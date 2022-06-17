package trie

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/pkg/common"
)

func TestKey(t *testing.T) {
	tests := []struct {
		hex      string
		expected []bool
	}{
		{"0x11", []bool{true, false, false, false, true}},
		{"0x11", []bool{false, false, true, false, false, false, true}},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestKeyCreate%d", i+1), func(t *testing.T) {
			key, err := BigToKey(len(test.expected), new(big.Int).SetBytes(common.FromHex(test.hex)))
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < key.Len(); i++ {
				if key.Get(i) != test.expected[i] {
					t.Errorf("key.Get(%d) == %v, want %v", i, key.Get(i), test.expected[i])
				}
			}

			key.Set(0)
			if !key.Get(0) {
				t.Errorf("key.Get(0) == %v, want %v", key.Get(0), true)
			}

			key.Clear(0)
			if key.Get(0) {
				t.Errorf("key.Get(0) == %v, want %v", key.Get(0), false)
			}

			key.Clear(0)
			if key.Get(0) {
				t.Errorf("key.Get(0) == %v, want %v", key.Get(0), false)
			}
		})
	}
}
