package cache

import (
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type TransactionStatusCache struct {
	m sync.Map
}

func NewTransactionStatusCache() *TransactionStatusCache {
	return &TransactionStatusCache{m: sync.Map{}}
}

func (t *TransactionStatusCache) Get(hash *felt.Felt) (any, bool) {
	return t.m.Load(hash)
}

func (t *TransactionStatusCache) Set(hash *felt.Felt, data any) {
	t.m.Store(hash, data)
}

func (t *TransactionStatusCache) OnTransactionStored(tx core.Transaction) {
	t.m.Delete(tx.Hash())
}
