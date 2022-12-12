package dataSource

import (
	"github.com/NethermindEth/juno/core"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type DataSource interface {
	GetBlockByNumber(blockNumber uint64) (*core.Block, error)
	GetTransaction(transactionHash *fp.Element) (*core.Transaction, error)
	GetClass(classHash *fp.Element) (*core.Class, error)
	GetStateUpdate(blockNumber uint64) (*core.StateUpdate, error)
}
