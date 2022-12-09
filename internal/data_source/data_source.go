package dataSource

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/contract"
	"github.com/NethermindEth/juno/core/felt"
)

type DataSource interface {
	GetBlockByNumber(blockNumber uint64) (*core.Block, error)
	GetTransaction(transactionHash *felt.Felt) (*core.Transaction, error)
	GetClass(classHash *felt.Felt) (*contract.Class, error)
	GetStateUpdate(blockNumber uint64) (*core.StateUpdate, error)
}
