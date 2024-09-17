package l1

import (
	"time"

	"github.com/NethermindEth/juno/core"
)

type EventListener interface {
	OnNewL1Head(head *core.L1Head)
	OnL1Call(method string, took time.Duration)
}

type SelectiveListener struct {
	OnNewL1HeadCb func(head *core.L1Head)
	OnL1CallCb    func(method string, took time.Duration)
}

func (l SelectiveListener) OnNewL1Head(head *core.L1Head) {
	if l.OnNewL1HeadCb != nil {
		l.OnNewL1HeadCb(head)
	}
}

func (l SelectiveListener) OnL1Call(method string, took time.Duration) {
	if l.OnL1CallCb != nil {
		l.OnL1CallCb(method, took)
	}
}
