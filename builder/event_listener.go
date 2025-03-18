package builder

import "github.com/NethermindEth/juno/core"

// Todo: What was this for again?

type EventListener interface {
	OnBlockFinalised(*core.Header)
}

type SelectiveListener struct {
	OnBlockFinalisedCb func(*core.Header)
}

func (l *SelectiveListener) OnBlockFinalised(h *core.Header) {
	if l.OnBlockFinalisedCb != nil {
		l.OnBlockFinalisedCb(h)
	}
}
