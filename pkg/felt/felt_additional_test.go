package felt

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
)

// These tests make up for the lack of coverage on felt.go provided by
// the generated tests in felt_test.go.

func TestNewFelt(t *testing.T) {
	want := &Felt{1, 0, 0, 0}
	got := NewFelt(1)
	if want.ToMont().Cmp(&got) != 0 {
		t.Error("Felt{1, 0, 0, 0} != NewFelt(1)")
	}
}

func TestSetInterface(t *testing.T) {
	type TestCase struct {
		inputType string
		input     interface{}
	}

	goodTests := []TestCase{
		{"*Felt", new(Felt).SetZero()},
		{"Felt", *new(Felt).SetZero()},
		{"*big.Int", new(big.Int)},
		{"big.Int", *new(big.Int)},
		{"string", "0"},
		{"[]byte", []byte{0}},
	}
	want := new(Felt).SetZero()
	for _, test := range goodTests {
		t.Run(fmt.Sprintf("should convert %s to *Felt", test.inputType), func(t *testing.T) {
			got, err := new(Felt).SetInterface(test.input)
			if err != nil {
				t.Errorf("unexpected error: %x", err)
			}
			if got.Cmp(want) != 0 {
				t.Errorf("got %x, want %x", got, want)
			}
		})
	}

	var f *Felt
	var b *big.Int
	errTests := []TestCase{
		{"nil felt pointer", f},
		{"nil big.Int pointer", b},
		{"unknown struct", struct{ malformed int }{malformed: 0}},
	}
	for _, test := range errTests {
		t.Run(fmt.Sprintf("should fail to convert %s", test.inputType), func(t *testing.T) {
			_, err := new(Felt).SetInterface(test.input)
			if err == nil {
				t.Error("conversion did not fail")
			}
		})
	}
}

func TestBit(t *testing.T) {
	getBit := uint64(10000)
	if new(Felt).Bit(getBit) != 0 {
		t.Errorf("Bit number %d should be zero", getBit)
	}
}

func TestIsUint64(t *testing.T) {
	input := new(Felt).SetUint64(3)
	if !input.IsUint64() {
		t.Errorf("felt %x can be represented as a uint64", input)
	}
}

func TestCmp(t *testing.T) {
	tests := [...]struct {
		inputs [2]*Felt
		want   int
	}{
		// z[2] > x[2]
		{
			inputs: [2]*Felt{{0, 0, 1, 0}, {0, 0, 0, 0}},
			want:   1,
		},
		// z[2] < x[2]
		{
			inputs: [2]*Felt{{0, 0, 0, 0}, {0, 0, 1, 0}},
			want:   -1,
		},
		// z[1] > x[1]
		{
			inputs: [2]*Felt{{0, 1, 0, 0}, {0, 0, 0, 0}},
			want:   1,
		},
		// z[1] < x[1]
		{
			inputs: [2]*Felt{{0, 0, 0, 0}, {0, 1, 0, 0}},
			want:   -1,
		},
		// z[0] > x[0]
		{
			inputs: [2]*Felt{{1, 0, 0, 0}, {0, 0, 0, 0}},
			want:   1,
		},
		// z[0] < x[0]
		{
			inputs: [2]*Felt{{0, 0, 0, 0}, {1, 0, 0, 0}},
			want:   -1,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("comparing %x and %x", test.inputs[0], test.inputs[1]), func(t *testing.T) {
			got := test.inputs[0].ToMont().Cmp(test.inputs[1].ToMont()) // Cmp will convert FromMont
			if got != test.want {
				t.Errorf("(%x).Cmp(%x) = %d, want %d", test.inputs[0], test.inputs[1], got, test.want)
			}
		})
	}
}

func TestMarshal(t *testing.T) {
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
		got := test.input.ToMont().Marshal() // Marshal converts FromMont
		if !bytes.Equal(got, test.want) {
			t.Errorf("ByteSlice(%x) = %x", got, test.want)
		}
	}
}

func TestSetBigInt(t *testing.T) {
	big65 := big.NewInt(65)
	tests := [...]struct {
		big  *big.Int
		want *Felt
	}{
		{
			big:  Modulus(),
			want: new(Felt),
		},
		{
			big:  new(big.Int).Exp(big.NewInt(2), big65, nil), // 2^65
			want: new(Felt).Exp(*Felt2, big65),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("set new(Felt) to %d", test.big), func(t *testing.T) {
			got := new(Felt).SetBigInt(test.big).Cmp(test.want)
			if got != 0 {
				t.Errorf("got %x, want %x", got, test.want)
			}
		})
	}
}

func TestBatchInvert(t *testing.T) {
	// NOTE: nothing is special about these test values
	data := []Felt{
		{0, 0, 0, 0},
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{182832, 2, 0, 0},
		{2, 3, 4, 0},
	}
	got := BatchInvert(data[:])
	for i, x := range data {
		_x := new(Felt).Set(&x)
		want := _x.Inverse(_x)
		if want.Cmp(&got[i]) != 0 {
			t.Errorf("got %x, want %x", got[i], want)
		}
	}

	if len(BatchInvert(make([]Felt, 0))) != 0 {
		t.Errorf("empty batch does not return empty")
	}
}

func TestUint64(t *testing.T) {
	got := new(Felt).SetOne().Uint64()
	want := uint64(1)
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestMulByConstant(t *testing.T) {
	got := new(Felt).SetOne()
	mulByConstant(got, uint8(11))
	want := new(Felt).SetUint64(11)
	if got.Cmp(want) != 0 {
		t.Errorf("1 * 11 = %x, want %x", got, want)
	}
}

func TestLegendre(t *testing.T) {
	if new(Felt).Legendre() != 0 {
		t.Error("legendre symbol of felt = 0 should be zero")
	}
}

func TestText(t *testing.T) {
	var z *Felt
	if z.Text(10) != "<nil>" {
		t.Errorf("nil *Felt does not return <nil> on Text()")
	}

	base := 37
	defer func() {
		if x := recover(); x == nil {
			t.Errorf("incorrect base does not panic")
		}
	}()
	new(Felt).Text(base)
}

func TestSetString(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Errorf("incorrect string param in SetString does not cause panic")
		}
	}()
	new(Felt).SetString("e")
}

func TestMulWSigned(t *testing.T) {
	new(Felt).mulWSigned(new(Felt), -1)
	// TODO figure out what this function is supposed to do?
}

func TestInverseExp(t *testing.T) {
	f := new(Felt)
	got := new(big.Int)
	f.inverseExp(f).ToBigIntRegular(got)
	b := new(big.Int)
	want := b.Exp(b, Modulus().Sub(Modulus(), big.NewInt(2)), Modulus())

	if got.Cmp(want) != 0 {
		t.Errorf("got %x, want %x", got, want)
	}
}
