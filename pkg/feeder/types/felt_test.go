package types

import (
	"math"
	"math/big"
	"strings"
	"testing"
)

func copyP() *big.Int {
	cp := new(big.Int).Set(p)
	return cp
}

func TestAsInt(t *testing.T) {
	var tests = [...]struct {
		input *Felt
		want  *big.Int
	}{
		{
			input: (*Felt)(big.NewInt(0)),
			want:  big.NewInt(0),
		},
		{
			input: (*Felt)(big.NewInt(23)),
			want:  big.NewInt(23),
		},
	}
	for _, test := range tests {
		result := test.input.asInt()
		if result.Cmp(test.want) != 0 {
			t.Errorf("asInt(%s) = %s, want %s", test.input, result.Text(16), test.want.Text(16))
		}
	}
}

func TestReduce(t *testing.T) {
	var tests = [...]struct {
		input *Felt
		want  *Felt
	}{
		{
			input: (*Felt)(big.NewInt(0)),
			want:  (*Felt)(big.NewInt(0)),
		},
		{
			input: (*Felt)(copyP()),
			want:  (*Felt)(big.NewInt(0)),
		},
		{
			input: (*Felt)(new(big.Int).Add(copyP(), big.NewInt(1))),
			want:  (*Felt)(big.NewInt(1)),
		},
	}
	for _, test := range tests {
		input := *test.input
		test.input.reduce()
		if test.input.Cmp(test.want) != 0 {
			t.Errorf("%s.reduce() = %s, want %s", &input, test.input, test.want)
		}
	}
}

func TestNewIn(t *testing.T) {
	var tests = [...]struct {
		input int64
		want  *Felt
	}{
		{
			input: 1,
			want:  (*Felt)(big.NewInt(1)),
		},
		{
			input: math.MaxInt64,
			want:  (*Felt)(big.NewInt(math.MaxInt64)),
		},
		{
			input: 10,
			want:  (*Felt)(big.NewInt(10)),
		},
	}
	for _, test := range tests {
		result := NewInt(test.input)
		if result.Cmp(test.want) != 0 {
			t.Errorf("NewInt(%d)=%s, want %s", test.input, result, test.want)
		}
	}
}

func TestSet(t *testing.T) {
	var tests = [...]struct {
		z, x *Felt
	}{
		{
			z: NewInt(0),
			x: NewInt(1),
		},
		{
			z: NewInt(2),
			x: NewInt(3),
		},
	}
	for _, test := range tests {
		test.z.Set(test.x)
		if test.z.Cmp(test.x) != 0 {
			t.Errorf("z.Set(%s)=%s, want %s", test.x, test.z, test.x)
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
			t.Errorf("ok=%t, want %t", ok, test.ok)
		}
		if ok && result.Cmp(test.want) != 0 {
			t.Errorf("SetString(%s)=%s, want %s", test.s, result, test.want)
		}
	}
}

func TestAdd(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{
			// 0 + 10 = 10
			a:    NewInt(0),
			b:    NewInt(10),
			want: NewInt(10),
		},
		{
			// (P - 1) + 10 = 9
			a:    (*Felt)(new(big.Int).Sub(copyP(), big.NewInt(1))),
			b:    NewInt(10),
			want: NewInt(9),
		},
		{
			// 3 + 3 = 6
			a:    NewInt(3),
			b:    NewInt(3),
			want: NewInt(6),
		},
	}
	for _, test := range tests {
		result := new(Felt).Add(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Add(%s, %s)=%s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestSub(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{
			// 0 - 1 = P - 1
			a:    NewInt(0),
			b:    NewInt(1),
			want: (*Felt)(new(big.Int).Sub(copyP(), big.NewInt(1))),
		},
		{
			// 3 - 3 = 0
			a:    NewInt(3),
			b:    NewInt(3),
			want: NewInt(0),
		},
		{
			// 10 - 20 = P - 10
			a:    NewInt(10),
			b:    NewInt(20),
			want: (*Felt)(new(big.Int).Sub(copyP(), big.NewInt(10))),
		},
		{
			// 10 - (P - 1) = 11
			a:    NewInt(10),
			b:    (*Felt)(new(big.Int).Sub(copyP(), big.NewInt(1))),
			want: NewInt(11),
		},
	}
	for _, test := range tests {
		result := new(Felt).Sub(test.a, test.b)
		t.Logf("Sub(%s, %s)=%s, want %s", test.a, test.b, result, test.want)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Sub(%s, %s)=%s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestMul(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{
			// 0 * 10 = 0
			a:    NewInt(0),
			b:    NewInt(10),
			want: NewInt(0),
		},
		{
			// 2 * 2 = 4
			a:    NewInt(2),
			b:    NewInt(2),
			want: NewInt(4),
		},
		{
			// 2 * ⎣P//2⎦ = P - 1
			a:    NewInt(2),
			b:    (*Felt)(new(big.Int).Div(copyP(), big.NewInt(2))),
			want: (*Felt)(new(big.Int).Sub(copyP(), big.NewInt(1))),
		},
	}
	for _, test := range tests {
		result := new(Felt).Mul(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Mul(%s, %s)=%s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestExp(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{
			// 2 ** 2 = 4
			a:    NewInt(2),
			b:    NewInt(2),
			want: NewInt(4),
		},
		{
			// 51 ** 0 = 1
			a:    NewInt(51),
			b:    NewInt(0),
			want: NewInt(1),
		},
		{
			// 0 ** 10000 = 0
			a:    NewInt(0),
			b:    NewInt(10000),
			want: NewInt(0),
		},
	}
	for _, test := range tests {
		result := new(Felt).Exp(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Exp(%s, %s)=%s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestDiv(t *testing.T) {
	var tests = [...]struct {
		a, b, want *Felt
	}{
		{
			// 6 / 3 = 2
			a:    NewInt(6),
			b:    NewInt(3),
			want: NewInt(2),
		},
		{
			// 7 / 3 = 2aaaaaaaaaaaab0555555555555555555555555555555555555555555555558
			a: NewInt(7),
			b: NewInt(3),
			want: func() *Felt {
				x, _ := new(big.Int).SetString("2aaaaaaaaaaaab0555555555555555555555555555555555555555555555558", 16)
				return (*Felt)(x)
			}(),
		},
	}
	for _, test := range tests {
		result := new(Felt).Div(test.a, test.b)
		if result.Cmp(test.want) != 0 {
			t.Errorf("Div(%s, %s)=%s, want %s", test.a, test.b, result, test.want)
		}
	}
}

func TestCmp(t *testing.T) {
	var tests = [...]struct {
		a, b *Felt
		want int
	}{
		{
			// 2 cmp 2 = 0
			a:    NewInt(2),
			b:    NewInt(2),
			want: 0,
		},
		{
			// 51 cmp 0 = 1
			a:    NewInt(51),
			b:    NewInt(0),
			want: 1,
		},
		{
			// 10 cmp 10000 = -1
			a:    NewInt(10),
			b:    NewInt(10000),
			want: -1,
		},
	}
	for _, test := range tests {
		result := test.a.Cmp(test.b)
		if result != test.want {
			t.Errorf("%s.Cmp(%s)=%d, want %d", test.a, test.b, result, test.want)
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
			t.Errorf("%s.Text(%d)=%s, want %s", test.a.asInt().Text(16), test.base, result, test.want)
		}
	}
}

func TestFeltUnmarshalJSON(t *testing.T) {
	var tests = [...]struct {
		data []byte
		felt *Felt
		err  bool
	}{
		{
			data: []byte("\"0xa\""),
			felt: NewInt(10),
			err:  false,
		},
		{
			data: []byte("\"10\""),
			felt: NewInt(10),
			err:  false,
		},
		{
			data: []byte("23"),
			felt: NewInt(23),
			err:  false,
		},
		{
			data: []byte("\"0xa"),
			felt: new(Felt),
			err:  true,
		},
		{
			data: []byte("\"0x2z4a\""),
			felt: new(Felt),
			err:  true,
		},
		{
			data: []byte("\"234a\""),
			felt: new(Felt),
			err:  true,
		},
		{
			data: []byte("234a\""),
			felt: new(Felt),
			err:  true,
		},
		{
			data: []byte("234a"),
			felt: new(Felt),
			err:  true,
		},
	}
	for _, test := range tests {
		f := new(Felt)
		err := f.UnmarshalJSON(test.data)
		if err != nil && !test.err {
			t.Errorf("unexpected error: %s", err.Error())
		}
		if err == nil && test.err {
			t.Errorf("expected error")
		}
		if f.Cmp(test.felt) != 0 {
			t.Errorf("unexpected felt %s unmarshaling data %v", f, test.data)
		}
	}
}

func TestString(t *testing.T) {
	var tests = [...]struct {
		felt *Felt
		want string
	}{
		{
			felt: NewInt(10),
			want: "0xa",
		},
		{
			felt: NewInt(0),
			want: "0x0",
		},
		{
			felt: NewInt(1000),
			want: "0x3e8",
		},
	}
	for _, test := range tests {
		result := test.felt.String()
		if strings.Compare(result, test.want) != 0 {
			t.Errorf("%s.String()=%s, want %s", test.felt.asInt().Text(16), result, test.want)
		}
	}
}
