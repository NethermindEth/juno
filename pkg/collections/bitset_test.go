package collections

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/pkg/common"
)

func TestBS(t *testing.T) {
	tests := []struct {
		hex      string
		expected []bool
	}{
		{"0x11", []bool{true, false, false, false, true}},
		{"0x11", []bool{false, false, true, false, false, false, true}},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestBSCreate%d", i+1), func(t *testing.T) {
			bs := NewBitSet(len(test.expected), common.FromHex(test.hex))
			for i := 0; i < bs.Len(); i++ {
				if bs.Get(i) != test.expected[i] {
					t.Errorf("bs.Get(%d) == %v, want %v", i, bs.Get(i), test.expected[i])
				}
			}

			bs.Set(0)
			if !bs.Get(0) {
				t.Errorf("bs.Get(0) == %v, want %v", bs.Get(0), true)
			}

			bs.Clear(0)
			if bs.Get(0) {
				t.Errorf("bs.Get(0) == %v, want %v", bs.Get(0), false)
			}

			bs.Clear(0)
			if bs.Get(0) {
				t.Errorf("bs.Get(0) == %v, want %v", bs.Get(0), false)
			}
		})
	}
}

func TestBitSet_Equals(t *testing.T) {
	tests := []struct {
		name string
		bs1  *BitSet
		bs2  *BitSet
		want bool
	}{
		{
			name: "equal",
			bs1:  NewBitSet(5, []byte{0x11}),
			bs2:  NewBitSet(5, []byte{0x11}),
			want: true,
		},
		{
			name: "not equal",
			bs1:  NewBitSet(5, []byte{0x11}),
			bs2:  NewBitSet(5, []byte{0x12}),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bs1.Equals(tt.bs2); got != tt.want {
				t.Errorf("BitSet.Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBitSet_String(t *testing.T) {
	tests := []struct {
		name string
		bs   *BitSet
		want string
	}{
		{
			name: "test 1",
			bs:   NewBitSet(5, []byte{0x11}),
			want: "10001",
		},
		{
			name: "test 2",
			bs:   NewBitSet(5, []byte{0x12}),
			want: "10010",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bs.String(); got != tt.want {
				t.Errorf("BitSet.String() = %s, want %s", got, tt.want)
			}
		})
	}
}
