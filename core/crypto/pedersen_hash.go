package crypto

import (
	"github.com/NethermindEth/juno/core/crypto/starkware"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// PedersenArray implements [Pedersen array hashing]
//
// [Pedersen array hashing]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func PedersenArray(elems ...*fp.Element) (*fp.Element, error) {
	var err error
	d := new(fp.Element)
	for _, e := range elems {
		d, err = Pedersen(d, e)
		if err != nil {
			return nil, err
		}
	}

	l, err := new(fp.Element).SetInterface(len(elems))
	if err != nil {
		return nil, err
	}

	return Pedersen(d, l)
}

// Pedersen implements the [Pedersen hash]
//
// [Pedersen hash]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func Pedersen(a *fp.Element, b *fp.Element) (*fp.Element, error) {
	out := make([]byte, starkware.BufferSize)
	if err := starkware.Hash(a.Marshal(), b.Marshal(), out); err != nil {
		return nil, err
	}
	return new(fp.Element).SetBytes(out[:starkware.FeltSize]), nil
}
