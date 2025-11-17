package utils

import (
	"math/big"
)

func ToHex(b *big.Int) string {
	return "0x" + b.Text(16)
}
