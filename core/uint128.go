package core

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
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

func (u *Uint128) UnmarshalJSON(data []byte) error {
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	switch v := value.(type) {
	case string:
		if v == "0x0" {
			u.hi, u.lo = 0, 0
			return nil
		}
		v = strings.TrimPrefix(v, "0x")

		// might need a leading zero
		if len(v)%2 != 0 {
			v = "0" + v
		}

		bytes, err := hex.DecodeString(v)
		if err != nil {
			return err
		}

		padSize := 16 - len(bytes)
		if padSize > 0 {
			padBytes := make([]byte, padSize)
			bytes = append(padBytes, bytes...)
		}

		u.hi = binary.BigEndian.Uint64(bytes[:8])
		u.lo = binary.BigEndian.Uint64(bytes[8:])
	default:
		return fmt.Errorf("unsupported type in JSON payload: %T", value)
	}

	return nil
}

func (u Uint128) Equal(o Uint128) bool {
	return u.hi == o.hi && u.lo == o.lo
}

func (u Uint128) String() string {
	return fmt.Sprintf("%016x%016x", u.hi, u.lo)
}
