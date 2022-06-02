package common

import (
	"encoding/hex"
	"math/big"
	"strings"
)

const (
	// FeltLength is the expected length of the felt
	FeltLength = 32
)

type Felt [FeltLength]byte

func BytesToFelt(b []byte) Felt {
	var f Felt
	f.SetBytes(b)
	return f
}

func BigToFelt(b *big.Int) Felt {
	return BytesToFelt(b.Bytes())
}

func HexToFelt(s string) Felt {
	return BytesToFelt(FromHex(s))
}

func (f Felt) Bytes() []byte {
	return f[:]
}

func (f Felt) Big() *big.Int {
	return new(big.Int).SetBytes(f[:])
}

func (f Felt) Hex() string {
	enc := make([]byte, len(f)*2)
	hex.Encode(enc, f[:])
	return "0x" + strings.TrimLeft(string(enc), "0")
}

func (f Felt) String() string {
	return f.Hex()
}

func (f *Felt) SetBytes(b []byte) {
	if len(b) > len(f) {
		b = b[len(b)-FeltLength:]
	}
	copy(f[FeltLength-len(b):], b)
}
