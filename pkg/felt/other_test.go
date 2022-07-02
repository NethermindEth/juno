package felt

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
)

func TestByteSlice(t *testing.T) {
	tests := [...]struct {
		input *Felt
		want  []byte
	}{
		{
			&Felt{10, 0, 0, 0},
			[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10},
		},
		{
			&Felt{12088959491439601242, 44, 0, 0},
			[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 44, 167, 196, 156, 155, 127, 88, 38, 90},
		},
	}

	for _, test := range tests {
		got := test.input.ToMont().ByteSlice() // ByteSlice converts FromMont
		if !bytes.Equal(got, test.want) {
			t.Errorf("ByteSlice(%x) = %x", got, test.want)
		}
	}
}

func TestHex(t *testing.T) {
	tests := [...]struct {
		input *Felt
		want  string
	}{
		{
			&Felt{10, 0, 0, 0},
			"a",
		},
		{
			&Felt{12088959491439601242, 44, 0, 0},
			"2ca7c49c9b7f58265a",
		},
		{
			&Felt{16942649892204837798, 3236446625469992744, 86143056028885, 0},
			"4e58be4124d52cea2b0ef5a68f28eb20624f9aadfba6",
		},
		{
			&Felt{13549574119996998610, 3447048780071762479, 1944, 256},
			"10000000000000007982fd6609a2b9efa2fbc09c1df983f43d2",
		},
	}

	for _, test := range tests {
		got := test.input.ToMont().Hex() // Hex converts FromMont
		if got != test.want {
			t.Errorf("SetHex(%s) = %s", got, test.want)
		}
	}
}

func TestSetHex(t *testing.T) {
	tests := [...]struct {
		input string
		want  *Felt
	}{
		{
			"0xa",
			&Felt{10, 0, 0, 0},
		},
		{
			"0x2ca7c49c9b7f58265a",
			&Felt{12088959491439601242, 44, 0, 0},
		},
		{
			"0x4e58be4124d52cea2b0ef5a68f28eb20624f9aadfba6",
			&Felt{16942649892204837798, 3236446625469992744, 86143056028885, 0},
		},
		{
			"0x10000000000000007982fd6609a2b9efa2fbc09c1df983f43d2",
			&Felt{13549574119996998610, 3447048780071762479, 1944, 256},
		},
	}

	got := new(Felt)
	for _, test := range tests {
		got.SetHex(test.input).FromMont()
		if got.Cmp(test.want) != 0 {
			t.Errorf("SetHex(%s) = %s", test.input, got.Hex())
			t.Errorf("%d, %d, %d, %d", got[0], got[1], got[2], got[3])
		}
	}
}

func TestSetBit(t *testing.T) {
	tests := [...]struct {
		z    *Felt
		bit  uint64
		val  uint64
		want *Felt
	}{
		{new(Felt).SetZero(), 0, 1, new(Felt).Set(Felt1)},
		{new(Felt).SetOne(), 0, 0, new(Felt).Set(Felt0)},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("SetBit test %d", i), func(t *testing.T) {
			test.z.FromMont().SetBit(test.bit, test.val)
			test.want.FromMont()
			if test.z.Cmp(test.want) != 0 {
				t.Errorf("SetBit(%d, %d) = %x, want %x", test.bit, test.val, test.z.Hex(), test.want.Hex())
			}
		})
	}
}

func TestToggleBit(t *testing.T) {
	maxUint64 := ^uint64(0)
	tests := [...]struct {
		val  *Felt
		bit  uint64
		want *Felt
	}{
		{new(Felt).SetZero(), 1, new(Felt).Set(Felt2)},
		{new(Felt).SetOne(), 1, new(Felt).Set(Felt3)},
		{new(Felt).SetUint64(maxUint64), 0, new(Felt).Sub(new(Felt).SetUint64(maxUint64), Felt1)},
		{new(Felt).SetUint64(maxUint64), 65, new(Felt).Add(new(Felt).SetUint64(maxUint64), new(Felt).Exp(*Felt2, new(Felt).SetUint64(65).ToBigIntRegular(new(big.Int))))},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("ToggleBit test %d", i), func(t *testing.T) {
			test.val.FromMont().ToggleBit(test.bit)
			test.want.FromMont()
			if test.val.Cmp(test.want) != 0 {
				t.Errorf("ToggleBit(%d) = %x, want %x", test.bit, test.val, test.want)
			}
		})
	}
}

// TestRsh is slightly modified from github.com/holiman/uint256. Most
// importantly, tests with negative numbers are removed.
func TestRsh(t *testing.T) {
	// these tests are taken from the same tests used for Rsh in math/big.Int
	tests := [...]struct {
		input string
		shift uint
		out   string
	}{
		{"0", 0, "0"},
		{"0", 1, "0"},
		{"0", 2, "0"},
		{"1", 0, "1"},
		{"1", 1, "0"},
		{"1", 2, "0"},
		{"2", 0, "2"},
		{"2", 1, "1"},
		{"4294967296", 0, "4294967296"},
		{"4294967296", 1, "2147483648"},
		{"4294967296", 2, "1073741824"},
		{"18446744073709551616", 0, "18446744073709551616"},
		{"18446744073709551616", 1, "9223372036854775808"},
		{"18446744073709551616", 2, "4611686018427387904"},
		{"18446744073709551616", 64, "1"},
		{"340282366920938463463374607431768211456", 64, "18446744073709551616"},
		{"340282366920938463463374607431768211456", 128, "1"},
	}

	actual := new(Felt)
	bigActual := new(big.Int)
	want := new(big.Int)
	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			actual.SetString(test.input)
			actual.Rsh(actual, test.shift)
			actual.ToBigIntRegular(bigActual)
			want.SetString(test.out, 10)
			if bigActual.Cmp(want) != 0 {
				t.Errorf("want := %x, got := %x", want, bigActual)
			}
		})
	}
}
