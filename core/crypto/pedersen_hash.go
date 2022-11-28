package crypto

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto/starkware"
	"github.com/NethermindEth/juno/core/felt"
)

// PedersenArray implements [Pedersen array hashing]
//
// [Pedersen array hashing]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func PedersenArray(elems ...*felt.Felt) (*felt.Felt, error) {
	if len(elems) < 3 {
		return nil, errors.New("number of elems must be more than 2")
	}

	var err error
	d := new(felt.Felt).SetZero()

	for _, e := range elems {
		d, err = Pedersen(d, e)
		if err != nil {
			return nil, err
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

func PedersenHash(elems ...*felt.Felt) (*felt.Felt, error) {
	if len(elems) < 2 {
		return nil, errors.New("number of elems must be 2 and above")
	}

	if len(elems) < 3 {
		return Pedersen(elems[0], elems[1])
	} else {
		return PedersenArray(elems...)
	}
}
