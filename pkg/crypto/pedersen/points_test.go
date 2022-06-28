package pedersen

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
	"github.com/NethermindEth/juno/pkg/felt"
)

// TestAdd does a basic test of the point.Add function.
func TestAdd(t *testing.T) {
	tests := [][]point{
		// (0, 0) + (1, 1)
		{
			point{new(felt.Felt).SetZero(), new(felt.Felt).SetZero()},
			point{new(felt.Felt).SetOne(), new(felt.Felt).SetOne()},
		},
		// (1, 1) + (0, 0)
		{
			point{new(felt.Felt).SetOne(), new(felt.Felt).SetOne()},
			point{new(felt.Felt).SetZero(), new(felt.Felt).SetZero()},
		},
		// (1, 1) + (1, 1)
		{
			point{new(felt.Felt).SetOne(), new(felt.Felt).SetOne()},
			point{new(felt.Felt).SetOne(), new(felt.Felt).SetOne()},
		},
		// (1, P - 1) + (1, 1)
		{
			point{new(felt.Felt).SetOne(), new(felt.Felt).Sub(felt.Felt0, felt.Felt1)},
			point{new(felt.Felt).SetOne(), new(felt.Felt).SetOne()},
		},
	}
	curve := weierstrass.Stark()
	for _, test := range tests {
		x0, y0 := new(big.Int), new(big.Int)
		test[0].x.ToBigIntRegular(x0)
		test[0].y.ToBigIntRegular(y0)

		x1, y1 := new(big.Int), new(big.Int)
		test[1].x.ToBigIntRegular(x1)
		test[1].y.ToBigIntRegular(y1)
		t.Run(fmt.Sprintf("(%d,%d).Add((%d,%d))", x0, y0, x1, y1), func(t *testing.T) {
			test[0].Add(&test[1])
			gotX, gotY := new(big.Int), new(big.Int)
			test[0].x.ToBigIntRegular(gotX)
			test[0].y.ToBigIntRegular(gotY)
			wantX, wantY := curve.Add(x0, y0, x1, y1)
			if gotX.Cmp(wantX) != 0 || gotY.Cmp(wantY) != 0 {
				t.Errorf("got (%d, %d), want (%d, %d)", gotX, gotY, wantX, wantY)
			}
		})
	}
}
