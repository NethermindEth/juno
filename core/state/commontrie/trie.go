package commontrie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type Trie interface {
	Update(key, value *felt.Felt) error
	Get(key *felt.Felt) (felt.Felt, error)
	Hash() (felt.Felt, error)
	HashFn() crypto.HashFn
}
