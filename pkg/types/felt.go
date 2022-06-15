package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

const (
	// FeltLength is the expected length of the felt in bytes
	FeltLength = 32
	// FeltBitLen is the expected length of the felt in bits
	FeltBitLen = 251
)

type IsFelt interface {
	Felt() Felt
}

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
	return BytesToFelt(common.FromHex(s))
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
	s := strings.TrimLeft(string(enc), "0")
	if s == "" {
		s = "0"
	}
	return "0x" + s
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

func (f Felt) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Hex())
}

func (f *Felt) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch t := token.(type) {
	case string:
		if !common.IsHex(t) {
			return errors.New("invalid hexadecimal string")
		}
		*f = HexToFelt(t)
	default:
		return errors.New("unexpected token type")
	}
	return nil
}

func (f Felt) Bit(i uint) uint {
	i += FeltLength*8 - FeltBitLen // convert to bit number
	return uint(f[i/8]>>(i%8)) & 1 // get bit
}

func (f Felt) SetBit(i uint, b uint) {
	i += FeltLength*8 - FeltBitLen // convert to bit number
	f[i/8] &^= 1 << (i % 8)        // clear bit
	f[i/8] |= byte(b) << (i % 8)   // set bit
}

// Felt arithmetic

func (f Felt) Add(g Felt) Felt {
	return BigToFelt(new(big.Int).Mod(new(big.Int).Add(f.Big(), g.Big()), FeltP.Big()))
}

// Felt as bit array

func (f Felt) Slice(start, end int) Felt {
	b := Felt0
	for i := start; i < end; i++ {
		b.SetBit(uint(i), f.Bit(uint(i)))
	}
	return b
}

func (f Felt) SliceFromStart(start int) Felt {
	b := Felt0
	for i := start; i < FeltBitLen; i++ {
		b.SetBit(uint(i), f.Bit(uint(i)))
	}
	return b
}

// Felt common

var (
	Felt0 = Felt{0}
	FeltP = BigToFelt(weierstrass.Stark().Params().P)
)
