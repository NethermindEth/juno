package uint128

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
)

// *Int[1] = high bits, more significant bits, upper 64 bits of a 128-bit value - ie [64:128]
// *Int[0] = low bits, less significant bits, lower 64 bits of a 128-bit value - ie [0:64]
// for example, if we have a 128-bit hex value 0x6e58133b38301a6cdfa34ca991c4ba39
// *Int[0] (lower bits) should be 0xdfa34ca991c4ba39 or dfa34ca991c4ba39
// *Int[i] (upper bits) should be 0x6e58133b38301a6c or 6e58133b38301a6c
// an example of a call to this type can be : uint128.Int([]uint64{lo, high})
type Int [2]uint64

const (
	Base16     = 16
	ByteLength = 16
	BitLength  = 128
)

var bigIntPool = sync.Pool{
	New: func() interface{} {
		return new(big.Int)
	},
}

func NewInt(x []uint64) (*Int, error) {
	if len(x) > 2 {
		return nil, fmt.Errorf("trying to add more than 128 bits to a 128-bit object")
	}

	res := bigIntPool.Get().(*big.Int)
	defer bigIntPool.Put(res)

	loBytes := make([]byte, 8)
	hiBytes := make([]byte, 8)

	binary.BigEndian.PutUint64(loBytes, x[0])
	binary.BigEndian.PutUint64(hiBytes, x[1])

	bytes := append(hiBytes, loBytes...)

	res.SetBytes(bytes)
	res.FillBytes(bytes[:])

	i := &Int{}

	return i.SetBigInt(res), nil
}

func (i *Int) SetBigInt(v *big.Int) *Int {
	// we're expecting words to have a length of 2, which represents 2x uint64s in a slice
	words := v.Bits()
        i[0] = words[0]
        i[1] = words[1]
	return i
}

func (i *Int) SetString(s string) (*Int, error) {
	vv := bigIntPool.Get().(*big.Int)
	defer bigIntPool.Put(vv)

	if _, ok := vv.SetString(s, 0); !ok {
		if _, ok := vv.SetString(s, Base16); !ok {
			return i, fmt.Errorf("could not parse string=%s into big.Int", s)
		}
	}

	bytes, err := parseHexString(s)

	if err != nil || len(bytes) > ByteLength {
		return nil, fmt.Errorf("can't fit string=%s into 128-bit uint; %s", s, err)
	}

	vv.FillBytes(bytes[:])

	return i.SetBigInt(vv), nil
}

func (i *Int) Bytes() []byte {
	res := make([]byte, ByteLength)

	binary.BigEndian.PutUint64(res[8:], i[0])
	binary.BigEndian.PutUint64(res[:8], i[1])

	return res
}

func (i *Int) UnmarshalJSON(data []byte) error {
	var v string

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	_, err := i.SetString(v)

	return err
}

func parseHexString(v string) ([]byte, error) {
	if v == "0x0" {
		return make([]byte, ByteLength), nil
	}

	v = strings.TrimPrefix(v, "0x")

	// might need a leading zero
	if len(v)%2 != 0 {
		v = "0" + v
	}

	bytes, err := hex.DecodeString(v)
	if err != nil {
		return nil, err
	}

	padSize := ByteLength - len(bytes)
	if padSize > 0 {
		padBytes := make([]byte, padSize)
		bytes = append(padBytes, bytes...)
	}

	return bytes, nil
}

func (i Int) Equal(o *Int) bool {
	return i[0] == o[0] && i[1] == o[1]
}

func (i *Int) Text(base int) string {
	if base < 2 || base > 36 {
		panic("invalid base")
	}

	vv := bigIntPool.Get().(*big.Int)
	defer bigIntPool.Put(vv)

	ii := *i
	b := ii.Bytes()
	vv.SetBytes(b[:])

	return vv.Text(base)
}

func (i Int) String() string {
	return "0x" + i.Text(Base16)
}

// MarshalJSON forwards the call to underlying field element implementation
func (i *Int) MarshalJSON() ([]byte, error) {
	return []byte("\"" + i.String() + "\""), nil
}

func (i *Int) MulWithFelt(x *felt.Felt, y *Int) *big.Int {
	bInt1 := bigIntPool.Get().(*big.Int)
	bInt2 := bigIntPool.Get().(*big.Int)
	bInt1 = x.BigInt(bInt1)
	bInt2.SetBytes(y.Bytes())

	return new(big.Int).Mul(bInt1, bInt2)
}
