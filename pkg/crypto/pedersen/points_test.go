package pedersen

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

// TestAdd does a basic test of the point.Add function.
func TestAdd(t *testing.T) {
	tests := [][]point{
		// (0, 0) + (1, 1)
		{
			point{big.NewInt(0), big.NewInt(0)},
			point{big.NewInt(1), big.NewInt(1)},
		},
		// (1, 1) + (0, 0)
		{
			point{big.NewInt(1), big.NewInt(1)},
			point{big.NewInt(0), big.NewInt(0)},
		},
		// (1, 1) + (1, 1)
		{
			point{big.NewInt(1), big.NewInt(1)},
			point{big.NewInt(1), big.NewInt(1)},
		},
		// (1, P - 1) + (1, 1)
		{
			point{big.NewInt(1), new(big.Int).Sub(prime, big.NewInt(1))},
			point{big.NewInt(1), big.NewInt(1)},
		},
	}
	curve := weierstrass.Stark()
	for _, test := range tests {
		t.Run(fmt.Sprintf("(%d,%d).Add((%d,%d))", test[0].x, test[0].y, test[1].x, test[1].y), func(t *testing.T) {
			x, y := new(big.Int).Set(test[0].x), new(big.Int).Set(test[0].y)
			test[0].add(&test[1])
			wantX, wantY := curve.Add(x, y, test[1].x, test[1].y)
			if test[0].x.Cmp(wantX) != 0 || test[0].y.Cmp(wantY) != 0 {
				t.Errorf("got (%d, %d), want (%d, %d)", test[0].x, test[0].y, wantX, wantY)
			}
		})
	}
}
