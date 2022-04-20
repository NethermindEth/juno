package felt

import (
	"math"
	"math/big"
	"strings"
	"testing"
)

// P returns a copy of the prime number 2²⁵¹ + 17·2¹⁹² + 1.
func P() *big.Int {
	copy := new(big.Int).Set(p)
	return copy
}

func TestInt(t *testing.T) {
	var tests = [...]struct {
		input *Felt
		want  *big.Int
	}{
		{(*Felt)(big.NewInt(0)), big.NewInt(0)},
		{(*Felt)(big.NewInt(23)), big.NewInt(23)},
	}
	for _, test := range tests {
		result := test.input.int()
		if result.Cmp(test.want) != 0 {
			t.Errorf("Felt.int() = %x, want %x", result, test.want)
		}
	}
}

func TestReduce(t *testing.T) {
	var tests = [...]struct {
		input *Felt
		want  *Felt
	}{
		{(*Felt)(big.NewInt(0)), (*Felt)(big.NewInt(0))},
		{(*Felt)(P()), (*Felt)(big.NewInt(0))},
		{(*Felt)(new(big.Int).Add(P(), big.NewInt(1))), (*Felt)(big.NewInt(1))},
	}
	for _, test := range tests {
		input := *test.input
		test.input.reduce()
		if test.input.Cmp(test.want) != 0 {
			t.Errorf("%s.reduce() = %s, want %s", &input, test.input, test.want)
		}
	}
}

func TestNew(t *testing.T) {
	var tests = [...]struct {
		input int64
		want  *Felt
	}{
		{1, (*Felt)(big.NewInt(1))},
		{math.MaxInt64, (*Felt)(big.NewInt(math.MaxInt64))},
		{10, (*Felt)(big.NewInt(10))},
	}
	for _, test := range tests {
		result := New(test.input)
		if result.Cmp(test.want) != 0 {
			t.Errorf("New(%d) = %s, want %s", test.input, result, test.want)
		}
	}
}

func TestSet(t *testing.T) {
	var tests = [...]struct {
		z, x *Felt
	}{
		{New(0), New(1)},
		{New(2), New(3)},
	}
	for _, test := range tests {
		test.z.Set(test.x)
		if test.z.Cmp(test.x) != 0 {
			t.Errorf("z.Set(%s) = %s, want %s", test.x, test.z, test.x)
		}
		if test.z == test.x {
			t.Error("z and x points to the same address after Set operation")
		}
	}
}

func TestSetString(t *testing.T) {
	var tests = [...]struct {
		s    string
		base int
		want *Felt
		ok   bool
	}{
		{
			s:    "3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb",
			base: 16,
			want: func() *Felt {
				v, _ := new(big.Int).SetString("3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb", 16)
				return (*Felt)(v)
			}(),
			ok: true,
		},
		{
			s:    "800000000000011000000000000000000000000000000000000000000000001",
			base: 16,
			want: (*Felt)(big.NewInt(0)),
			ok:   true,
		},
		{
			s:    "0x800000000000011000000000000000000000000000000000000000000000001",
			base: 16,
			want: (*Felt)(big.NewInt(0)),
			ok:   false,
		},
	}
	for _, test := range tests {
		result, ok := new(Felt).SetString(test.s, test.base)
		if ok != test.ok {
			t.Errorf("ok = %t, want %t", ok, test.ok)
		}
		if ok && result.Cmp(test.want) != 0 {
			t.Errorf("SetString(%s, %d) = %s, want %s", test.s, test.base, result, test.want)
		}
	}
}

func TestAdd(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{New(0), New(10), New(10)},
		{(*Felt)(new(big.Int).Sub(P(), big.NewInt(1))), New(10), New(9)},
		{New(3), New(3), New(6)},
	}
	for _, test := range tests {
		result := new(Felt).Add(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Add(%s, %s) = %s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestSub(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{New(0), New(1), (*Felt)(new(big.Int).Sub(P(), big.NewInt(1)))},
		{New(3), New(3), New(0)},
		{New(10), New(20), (*Felt)(new(big.Int).Sub(P(), big.NewInt(10)))},
		{New(10), (*Felt)(new(big.Int).Sub(P(), big.NewInt(1))), New(11)},
	}
	for _, test := range tests {
		result := new(Felt).Sub(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Sub(%s, %s) = %s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestMul(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{New(0), New(10), New(0)},
		{New(2), New(2), New(4)},
		{
			// 2 * ⎣P / 2⎦ = P - 1.
			New(2),
			(*Felt)(new(big.Int).Div(P(), big.NewInt(2))),
			(*Felt)(new(big.Int).Sub(P(), big.NewInt(1))),
		},
	}
	for _, test := range tests {
		result := new(Felt).Mul(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Mul(%s, %s) = %s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestExp(t *testing.T) {
	// a ** b.
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{New(2), New(2), New(4)},
		{New(51), New(0), New(1)},
		{New(0), New(10_000), New(0)},
	}
	for _, test := range tests {
		result := new(Felt).Exp(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Exp(%s, %s) = %s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestDiv(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{New(6), New(3), New(2)},
		{
			a: New(7),
			b: New(3),
			want: func() *Felt {
				x, _ := new(big.Int).SetString("2aaaaaaaaaaaab0555555555555555555555555555555555555555555555558", 16)
				return (*Felt)(x)
			}(),
		},
	}
	for _, test := range tests {
		result := new(Felt).Div(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Div(%s, %s) = %s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestCmp(t *testing.T) {
	var tests = [...]struct {
		a, b *Felt
		want int
	}{
		{New(2), New(2), 0},
		{New(51), New(0), 1},
		{New(10), New(10_000), -1},
	}
	for _, test := range tests {
		result := test.a.Cmp(test.b)
		if result != test.want {
			t.Errorf("%s.Cmp(%s) = %d, want %d", test.a, test.b, result, test.want)
		}
	}
}

func TestText(t *testing.T) {
	var tests = [...]struct {
		a    *Felt
		base int
		want string
	}{
		{
			a: func() *Felt {
				x, _ := new(Felt).SetString("3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb", 16)
				return x
			}(),
			base: 10,
			want: "1740729136829561885683894917751815192814966525555656371386868611731128807883",
		},
		{
			a: func() *Felt {
				x, _ := new(Felt).SetString("1740729136829561885683894917751815192814966525555656371386868611731128807883", 10)
				return x
			}(),
			base: 16,
			want: "3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb",
		},
	}
	for _, test := range tests {
		result := test.a.Text(test.base)
		if strings.Compare(result, test.want) != 0 {
			t.Errorf("%s.Text(%d) = %s, want %s", test.a.int().Text(16), test.base, result, test.want)
		}
	}
}

func TestFeltUnmarshalJSON(t *testing.T) {
	var tests = [...]struct {
		data []byte
		felt *Felt
		err  bool
	}{
		{[]byte("\"0xa\""), New(10), false},
		{[]byte("\"10\""), New(10), false},
		{[]byte("23"), New(23), false},
		{[]byte("\"0xa"), new(Felt), true},
		{[]byte("\"0x2z4a\""), new(Felt), true},
		{[]byte("\"234a\""), new(Felt), true},
		{[]byte("234a\""), new(Felt), true},
		{[]byte("234a"), new(Felt), true},
	}
	for _, test := range tests {
		f := new(Felt)
		err := f.UnmarshalJSON(test.data)
		if err != nil && !test.err {
			t.Errorf("unexpected error: %v", err)
		}
		if err == nil && test.err {
			t.Errorf("expected error: %v", err)
		}
		if f.Cmp(test.felt) != 0 {
			t.Errorf("unexpected felt %s unmarshaling data %v", f, test.data)
		}
	}
}

// XXX: I don't think we should confine ourselves to the hexadecimal
// representation so this test should reflect should reflect the ability
// to format against different printf specifiers instead.

func TestString(t *testing.T) {
	var tests = [...]struct {
		felt *Felt
		want string
	}{
		{
			felt: New(10),
			want: "a",
		},
		{
			felt: New(0),
			want: "0",
		},
		{
			felt: New(1000),
			want: "3e8",
		},
	}
	for _, test := range tests {
		result := test.felt.String()
		if strings.Compare(result, test.want) != 0 {
			t.Errorf("%s.String() = %s, want %s", test.felt.int().Text(16), result, test.want)
		}
	}
}
