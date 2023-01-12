package starknetdata

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type StarkNetData interface {
	GetBlockByNumber(blockNumber uint64) (*core.Block, error)
	GetTransaction(transactionHash *felt.Felt) (*core.Transaction, error)
	GetClass(classHash *felt.Felt) (*core.Class, error)
	GetStateUpdate(blockNumber uint64) (*core.StateUpdate, error)
}
