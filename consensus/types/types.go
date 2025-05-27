package types

import "github.com/NethermindEth/juno/core/felt"

// Hashable's Hash() is used as ID()
type Hash felt.Felt

type Addr felt.Felt

func (a *Addr) SetUint64(v uint64) {
	(*felt.Felt)(a).SetUint64(v)
}

func (a *Addr) Equal(b *Addr) bool {
	return (*felt.Felt)(a).Equal((*felt.Felt)(b))
}

type Hashable interface {
	Hash() Hash
}
