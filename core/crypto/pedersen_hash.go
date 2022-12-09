package crypto

import (
	"github.com/NethermindEth/juno/core/crypto/starkware"
	"github.com/NethermindEth/juno/core/felt"
)

// PedersenArray implements [Pedersen array hashing]
//
// [Pedersen array hashing]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func PedersenArray(elems ...*felt.Felt) (*felt.Felt, error) {
	var err error
	d := new(felt.Felt).SetZero()

	if len(elems) > 0 {
		for _, e := range elems {
			d, err = Pedersen(d, e)
			if err != nil {
				return nil, err
			}
		}
	}

	l, err := new(felt.Felt).SetInterface(len(elems))
	if err != nil {
		return nil, err
	}

	return Pedersen(d, l)
}

// Pedersen implements the [Pedersen hash]
//
// [Pedersen hash]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func Pedersen(a *felt.Felt, b *felt.Felt) (*felt.Felt, error) {
	out := make([]byte, starkware.BufferSize)
	if err := starkware.Hash(a.Marshal(), b.Marshal(), out); err != nil {
		return nil, err
	}
	return new(felt.Felt).SetBytes(out[:starkware.FeltSize]), nil
}
