package sync

import "github.com/NethermindEth/juno/core"

type Observer interface {
	OnTransactionStored(tx core.Transaction)
}

type NopObserver struct{}

func (n NopObserver) OnTransactionStored(core.Transaction) {}
