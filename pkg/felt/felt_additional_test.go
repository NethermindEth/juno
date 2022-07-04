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
	want := new(Felt).SetZero()

	inputFeltPtr := new(Felt).SetZero()
	gotFeltPtr, err := new(Felt).SetInterface(inputFeltPtr)
	if err != nil {
		t.Errorf("unexpected error on new(Felt).SetInterface(%x): %x", *inputFeltPtr, err)
	}
	if gotFeltPtr.Cmp(want) != 0 {
		t.Errorf("set felt to *Felt: got %x, want %x", gotFeltPtr, want)
	}

	inputFelt := *new(Felt).SetZero()
	gotFelt, err := new(Felt).SetInterface(inputFelt)
	if err != nil {
		t.Errorf("unexpected error on new(Felt).SetInterface(%x): %x", inputFelt, err)
	}
	if gotFelt.Cmp(want) != 0 {
		t.Errorf("set felt to Felt: got %x, want %x", gotFelt, want)
	}

	var f *Felt
	_, err = new(Felt).SetInterface(f)
	if err == nil {
		t.Error("expected error when setting felt to nil felt pointer")
	}

	inputString := "0"
	gotString, err := new(Felt).SetInterface(inputString)
	if err != nil {
		t.Errorf("unexpected error on new(Felt).SetInterface(%s): %x", inputString, err)
	}
	if gotString.Cmp(want) != 0 {
		t.Errorf("set felt to string: got %x, want %x", gotString, want)
	}

	inputBigInt := new(big.Int)
	gotBigInt, err := new(Felt).SetInterface(inputBigInt)
	if err != nil {
		t.Errorf("unexpected error on new(Felt).SetInterface(%x): %x", inputBigInt, err)
	}
	if gotBigInt.Cmp(want) != 0 {
		t.Errorf("set felt to big.Int: got %x, want %x", gotBigInt, want)
	}

	inputBytes := [32]byte{0}
	gotBytes, err := new(Felt).SetInterface(inputBytes[:])
	if err != nil {
		t.Errorf("unexpected error on new(Felt).SetInterface(%x): %x", inputBytes, err)
	}
	if gotBytes.Cmp(want) != 0 {
		t.Errorf("set felt to []byte: got %x, want %x", gotBytes, want)
	}

	inputUnexpected := struct{ malformed int }{malformed: 0}
	_, err = new(Felt).SetInterface(inputUnexpected)
	if err == nil {
		t.Errorf("expected err on new(Felt).SetInterface(%x)", inputUnexpected)
	}
}

func TestBit(t *testing.T) {
	getBit := uint64(Bits + 1)
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
		// z[0] < x[0]
		{
			inputs: [2]*Felt{{0, 0, 0, 0}, {1, 0, 0, 0}},
			want:   -1,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("comparing %x and %x", test.inputs[0], test.inputs[1]), func(t *testing.T) {
			got := test.inputs[0].Cmp(test.inputs[1])
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
}
