package application

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type block core.Block

// Todo
func (b block) Hash() felt.Felt {
	return *b.Header.Hash
}

type transaction struct {
	Transaction core.Transaction
	index       int // to order txns in the block
}
