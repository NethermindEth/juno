package l1

import (
	"github.com/NethermindEth/juno/core"
)

type EventListener interface {
	OnNewL1Head(head *core.L1Head)
}

type SelectiveListener struct {
	OnNewL1HeadCb func(head *core.L1Head)
}

func (l SelectiveListener) OnNewL1Head(head *core.L1Head) {
	if l.OnNewL1HeadCb != nil {
		l.OnNewL1HeadCb(head)
	}
}
