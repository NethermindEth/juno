package p2p2core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptHash(hash *spec.Hash) (felt.Felt, error) {
	elems := hash.GetElements()
	if elems == nil {
		return felt.Zero, errors.New("nil elements")
	}
	if len(elems) > felt.Bytes {
		return felt.Zero, errors.New("elems dont fit in felt")
	}

	var f felt.Felt
	f.SetBytes(elems)
	return f, nil
}
