package uint128

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
)

// *Int[1] = high bits, more significant bits, upper 64 bits of a 128-bit value - ie [64:128]
// *Int[0] = low bits, less significant bits, lower 64 bits of a 128-bit value - ie [0:64]
// for example, if we have a 128-bit hex value 0x6e58133b38301a6cdfa34ca991c4ba39
// *Int[0] (lower bits) should be 0xdfa34ca991c4ba39 or dfa34ca991c4ba39
// *Int[i] (upper bits) should be 0x6e58133b38301a6c or 6e58133b38301a6c
// an example of a call to this type can be : uint128.Int([]uint64{lo, high})
type Int [2]uint64

const (
	Base16 = 16
	Bytes  = 16
	Bits   = 128
)

var bigIntPool = sync.Pool{
	New: func() interface{} {
		return new(big.Int)
	},
}

func (i *Int) SetBigInt(v *big.Int) *Int {
	words := v.Bits()

	if len(words) > 0 {
		switch {
		case len(words)%2 == 0:
			i[0] = uint64(words[0])
			i[1] = uint64(words[1])
		case len(words)%2 == 1:
			i[0] = uint64(words[0])
		}
	}

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

	if vv.BitLen() > Bits {
		return nil, fmt.Errorf("can't fit string=%s into 128-bit uint", s)
	}

	return i.SetBigInt(vv), nil
}

func (i *Int) Bytes() []byte {
	res := make([]byte, Bytes)

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

func (i *Int) MarshalJSON() ([]byte, error) {
	return []byte("\"" + i.String() + "\""), nil
}
