package felt

import (
	"fmt"

	"github.com/NethermindEth/juno/pkg/common"
)

// ByteSlice() is exactly the same as Marshal() except that it returns
// null when z is null. This mimics the behavior of big.Int.
func (z *Felt) ByteSlice() []byte {
	if z == nil {
		// notest
		return nil
	}
	return z.Marshal()
}

// SetHex sets z to the value represented by the hex string s mod q.
// It makes no difference if there is a leading 0x prefix on s. This
// function assumes s is in regular form and will convert to Montgomery
// form.
func (z *Felt) SetHex(s string) *Felt {
	return z.SetBytes(common.FromHex(s))
}

// Hex returns a hex string without the 0x prefix. This function will
// automatically convert to regular form.
func (z *Felt) Hex() string {
	return z.Text(16)
}

func (z *Felt) Hex0x() string {
	return fmt.Sprintf("0x0%063s", z.Hex())
}

// SetBit sets bit i to j on z. Undefined behavior if
// j is not zero or one. It is the responsibility of the caller to
// convert to/from regular form if necessary.
func (z *Felt) SetBit(i uint64, j uint64) *Felt {
	if z.Bit(i) != j {
		return z.ToggleBit(i)
	}
	return z
}

// ToggleBit flips bit i on z and returns z. Returns z if i is greater
// than the number of bits than Felt can represent. It is the
// responsibility of the caller to convert to/from regular form if
// necessary.
func (z *Felt) ToggleBit(i uint64) *Felt {
	idx := i / 64
	if idx >= Limbs {
		return z
	}
	val := uint64(1 << (i - idx*64)) // 2^(bit - idx*64)
	if z.Bit(i) == 1 {
		z[idx] -= val
	} else {
		z[idx] += val
	}
	return z
}

// CmpCompat is exactly the same as Cmp except that it returns zero if
// both z and x are nil.
func (z *Felt) CmpCompat(x *Felt) int {
	if z == x { // Handles nils similar to big.Int.Cmp
		return 0
	}
	return z.Cmp(x)
}

// Everything below is slightly modified from github.com/holiman/uint256

func (z *Felt) rsh64(x *Felt) *Felt {
	z[3], z[2], z[1], z[0] = 0, x[3], x[2], x[1]
	return z
}

func (z *Felt) rsh128(x *Felt) *Felt {
	z[3], z[2], z[1], z[0] = 0, 0, x[3], x[2]
	return z
}

func (z *Felt) rsh192(x *Felt) *Felt {
	z[3], z[2], z[1], z[0] = 0, 0, 0, x[3]
	return z
}

// Rsh sets z = x >> n and returns z.
func (z *Felt) Rsh(x *Felt, n uint) *Felt {
	__x := *x
	_x := &__x
	_x.FromMont()
	// n % 64 == 0
	if n&0x3f == 0 {
		switch n {
		case 0:
			return z.Set(_x).ToMont()
		case 64:
			return z.rsh64(_x).ToMont()
		case 128:
			return z.rsh128(_x).ToMont()
		case 192:
			return z.rsh192(_x).ToMont()
		default:
			return z.SetZero()
		}
	}
	var a, b uint64
	// Big swaps first
	switch {
	case n > 192:
		if n > Bits {
			return z.SetZero()
		}
		z.rsh192(_x)
		n -= 192
		goto sh192
	case n > 128:
		z.rsh128(_x)
		n -= 128
		goto sh128
	case n > 64:
		z.rsh64(_x)
		n -= 64
		goto sh64
	default:
		z.Set(_x)
	}

	// remaining shifts
	a = z[3] << (64 - n)
	z[3] = z[3] >> n

sh64:
	b = z[2] << (64 - n)
	z[2] = (z[2] >> n) | a

sh128:
	a = z[1] << (64 - n)
	z[1] = (z[1] >> n) | b

sh192:
	z[0] = (z[0] >> n) | a

	return z.ToMont()
}
