package felt

import (
	"fmt"
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
			f:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}},
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
				fmt.Printf("got: %v\n", got)
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
			f:    Felt{val: fp.Element{18446744073709551585, 18446744073709551615, 18446744073709551615, 576460752303422960}},
			want: Felt{val: fp.Element{18446744073709551601, 18446744073709551615, 18446744073709551615, 576460752303423232}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f.Halve()
			fmt.Printf("\ntt.f: %v\n\n", tt.want.val.String())
			if !tt.f.Equal(&tt.want) {
				t.Errorf("Felt.Halve() = %v, want %v", tt.f, tt.want)
			}
		})
	}
}
