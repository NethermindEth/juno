package felt

import (
	"reflect"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalJson(t *testing.T) {
	var with Felt
	assert.NoError(t, with.UnmarshalJSON([]byte("0x4437ab")))

	var without Felt
	assert.NoError(t, without.UnmarshalJSON([]byte("4437ab")))
	assert.Equal(t, true, without.Equal(&with))
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name string
		f    Felt
		want bool
	}{
		{
			name: "zero",
			f:    Felt{},
			want: true,
		},
		{
			name: "non zero",
			f:    Felt{val: fp.Element{1, 2, 3, 4}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.IsZero(); got != tt.want {
				t.Errorf("Felt.IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOne(t *testing.T) {
	tests := []struct {
		name string
		f    Felt
		want bool
	}{
		{
			name: "one",
			f:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			want: true,
		},
		{
			name: "non one",
			f:    Felt{val: fp.Element{1, 2, 3, 4}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.IsOne(); got != tt.want {
				t.Errorf("Felt.IsOne() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHalve(t *testing.T) {
	tests := []struct {
		name string
		f    Felt
		want Felt
	}{
		{
			name: "zero halved",
			f:    Felt{val: fp.Element{0, 0, 0, 0}},
			want: Felt{val: fp.Element{0, 0, 0, 0}},
		},
		{
			name: "one halved",
			f:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			want: Felt{val: fp.Element{18446744073709551601, 18446744073709551615, 18446744073709551615, 576460752303423232}}, // 1/2
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f.Halve()
			if !tt.f.Equal(&tt.want) {
				t.Errorf("Felt.Halve() = %v, want %v", tt.f, tt.want)
			}
		})
	}
}

func TestSetUint64(t *testing.T) {
	tests := []struct {
		name   string
		f      Felt
		setVal uint64
		want   Felt
	}{
		{
			name:   "zero",
			f:      Felt{},
			setVal: 0,
			want:   Felt{val: fp.Element{0, 0, 0, 0}},
		},
		{
			name:   "one",
			f:      Felt{},
			setVal: 1,
			want:   Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f.SetUint64(tt.setVal)
			if !tt.f.Equal(&tt.want) {
				t.Errorf("Felt.SetUint64() = %v, want %v", tt.f, tt.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		f    Felt
		g    Felt
		want bool
	}{
		{
			name: "equal",
			f:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			g:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			want: true,
		},
		{
			name: "not equal",
			f:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			g:    Felt{val: fp.Element{18446744073709551553, 18446744073709551615, 18446744073709551615, 576460752303422416}}, // 2
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Equal(&tt.g); got != tt.want {
				t.Errorf("Felt.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		f       Felt
		want    []byte
		wantErr bool
	}{
		{
			name:    "zero",
			f:       Felt{val: fp.Element{0, 0, 0, 0}},
			want:    []byte("0"),
			wantErr: false,
		},
		{
			name:    "one",
			f:       Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			want:    []byte("1"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("Felt.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Felt.MarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name string
		f    Felt
		g    Felt
		h    Felt
		want Felt
	}{
		{
			name: "zero",
			f:    Felt{},
			g:    Felt{val: fp.Element{0, 0, 0, 0}},
			h:    Felt{val: fp.Element{0, 0, 0, 0}},
			want: Felt{val: fp.Element{0, 0, 0, 0}},
		},
		{
			name: "one plus one",
			f:    Felt{},
			g:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			h:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}}, // 1
			want: Felt{val: fp.Element{18446744073709551553, 18446744073709551615, 18446744073709551615, 576460752303422416}}, // 2
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f.Add(&tt.g, &tt.h)
			if !tt.f.Equal(&tt.want) {
				t.Errorf("Felt.Add() = %v, want %v", tt.f, tt.want)
			}
		})
	}
}
