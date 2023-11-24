package uint128

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Int [2]uint64

func NewInt64(lo uint64) *Int {
	// i[1] = high bits
	// i[0] = low bits
	z := &Int{}
	z.setUint128(0, lo)
	return z
}
func NewInt128(hi, lo uint64) *Int {
	z := &Int{}
	z.setUint128(hi, lo)
	return z
}

func NewUint128(hex_str string) *Int {
	z := &Int{}

	z.parseHexString(hex_str)

	return z
}

func (i *Int) setUint128(hi, lo uint64) error {
	loBytes := make([]byte, 8)
	hiBytes := make([]byte, 8)

	binary.BigEndian.PutUint64(loBytes, lo)
	binary.BigEndian.PutUint64(hiBytes, hi)

	err := i.setBytes(append(hiBytes, loBytes...))
	if err != nil {
		return fmt.Errorf("couldn't marshal %016x%016x into a uint128 type", lo, hi)
	}

	return nil
}

func (i *Int) Bytes() []byte {
	hiBytes := make([]byte, 8)
	loBytes := make([]byte, 8)

	binary.BigEndian.PutUint64(hiBytes, i[1])
	binary.BigEndian.PutUint64(loBytes, i[0])

	return append(hiBytes, loBytes...)
}

func (i *Int) setBytes(buf []byte) error {
	if len(buf) > 16 {
		return fmt.Errorf("trying to set bytes larger than 16 bytes (128-bits) to a 128-bit container")
	}

	_ = buf[len(buf)-1] // bounds check hint to compiler; see golang.org/issue/14808

	// upper 8 bytes / 64 bits
	i[1] = binary.BigEndian.Uint64(buf[:8])
	// lower 8 bytes / 64 bits
	i[0] = binary.BigEndian.Uint64(buf[8:])

	return nil
}

func (i *Int) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	if err := i.parseHexString(v); err != nil {
		return err
	}

	return nil
}

func (i *Int) parseHexString(v string) error {
	if v == "0x0" {
		i[1], i[0] = 0, 0
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

	i[1] = binary.BigEndian.Uint64(bytes[:8])
	i[0] = binary.BigEndian.Uint64(bytes[8:])

	return nil
}

func (i Int) Equal(o *Int) bool {
	return i[1] == o[1] && i[0] == o[0]
}

func (i Int) String() string {
	return fmt.Sprintf("%016x%016x", i[1], i[0])
}
