package p2p

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/pkg/errors"
)

// ClassProvider When converting to proto, the class is needed.
type ClassProvider interface {
	GetClass(hash *felt.Felt) (*core.DeclaredClass, error)
}

var _ ClassProvider = &blockchainClassProvider{}

type blockchainClassProvider struct {
	blockchain *blockchain.Blockchain
}

func (b *blockchainClassProvider) GetClass(hash *felt.Felt) (*core.DeclaredClass, error) {
	headStateReader, closer, err := b.blockchain.HeadState()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get head state")
	}

	defer func() {
		_ = closer()
	}()

	return headStateReader.Class(hash)
}
