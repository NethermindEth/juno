package felt

import (
	"errors"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type Felt struct {
	*fp.Element
}

var bigIntPool = sync.Pool{
	New: func() interface{} {
		return new(big.Int)
	},
}

// UnmarshalJSON accepts numbers and strings as input.
// See Element.SetString for valid prefixes (0x, 0b, ...).
// If there is an error, we try to explicitly unmarshal from hex before
// returning an error. This implementation is taken from [gnark-crypto].
//
// [gnark-crypto]: https://github.com/ConsenSys/gnark-crypto/blob/9fd0a7de2044f088a29cfac373da73d868230148/ecc/stark-curve/fp/element.go#L1028-L1056
func (z *Felt) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) > fp.Bits*3 {
		return errors.New("value too large (max = Element.Bits * 3)")
	}

	// we accept numbers and strings, remove leading and trailing quotes if any
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}

	// get temporary big int from the pool
	vv := bigIntPool.Get().(*big.Int)

	if _, ok := vv.SetString(s, 0); !ok {
		if _, ok := vv.SetString(s, 16); !ok {
			return errors.New("can't parse into a big.Int: " + s)
		}
	}

	z.SetBigInt(vv)

	// release object into pool
	bigIntPool.Put(vv)
	return nil
}
