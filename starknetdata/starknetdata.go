package starknetdata

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// StarknetData defines the function which are required to retrieve Starknet's state
type StarknetData interface {
	BlockByNumber(blockNumber uint64) (*core.Block, error)
	Transaction(transactionHash *felt.Felt) (core.Transaction, error)
	Class(classHash *felt.Felt) (*core.Class, error)
	StateUpdate(blockNumber uint64) (*core.StateUpdate, error)
}
