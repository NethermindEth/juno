package pedersen

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

func TestAdd(t *testing.T) {
	var tests = [][]point{
		// (0, 0) + (0, 0)
		{
			point{big.NewInt(0), big.NewInt(0)},
			point{big.NewInt(0), big.NewInt(0)},
		},
		// (0, 0) + (3, 0)
		{
			point{big.NewInt(0), big.NewInt(0)},
			point{big.NewInt(3), big.NewInt(0)},
		},
		// (3, 0) + (0, 0)
		{
			point{big.NewInt(3), big.NewInt(0)},
			point{big.NewInt(0), big.NewInt(0)},
		},
		{
			point{big.NewInt(1), big.NewInt(2)},
			point{big.NewInt(3), big.NewInt(4)},
		},
		{
			// p - 1
			point{new(big.Int).Sub(P, big.NewInt(1)), big.NewInt(10)},
			point{big.NewInt(1), big.NewInt(5)},
		},
		{
			point{big.NewInt(1), big.NewInt(10)},
			// p - 1
			point{new(big.Int).Sub(P, big.NewInt(1)), big.NewInt(5)},
		},
		{
			point{big.NewInt(1), big.NewInt(10)},
			point{new(big.Int).Set(P), big.NewInt(5)},
		},
		{
			point{new(big.Int).Set(P), big.NewInt(5)},
			point{big.NewInt(1), big.NewInt(10)},
		},
	}
	curve := weierstrass.Stark()
	for _, test := range tests {
		t.Run(fmt.Sprintf("(%d,%d).Add((%d,%d))", test[0].x, test[0].y, test[1].x, test[1].y), func(t *testing.T) {
			x, y := new(big.Int).Set(test[0].x), new(big.Int).Set(test[0].y)
			test[0].Add(test[1])
			wantX, wantY := curve.Add(x, y, test[1].x, test[1].y)
			if test[0].x.Cmp(wantX) != 0 || test[0].y.Cmp(wantY) != 0 {
				t.Errorf("got (%d, %d), want (%d, %d)", test[0].x, test[0].y, wantX, wantY)
			}
		})
	}
}
