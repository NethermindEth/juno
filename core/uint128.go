package core

import (
	"encoding/binary"
)

type Uint128 struct {
	hi uint64
	lo uint64
}

func NewUint128(hi, lo uint64) *Uint128 {
	return &Uint128{
		hi: hi,
		lo: lo,
	}
}

func (u *Uint128) Bytes() []byte {
	hiBytes := make([]byte, 8)
	loBytes := make([]byte, 8)

	binary.BigEndian.PutUint64(hiBytes, u.hi)
	binary.BigEndian.PutUint64(loBytes, u.lo)

	return append(hiBytes, loBytes...)
}
