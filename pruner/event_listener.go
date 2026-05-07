package pruner

import "time"

type EventListener interface {
	OnPrune(oldestBlockKept uint64, blocksPruned uint64, took time.Duration)
	OnPruneError(err error)
}

type SelectiveListener struct {
	OnPruneCb      func(oldestBlockKept uint64, blocksPruned uint64, took time.Duration)
	OnPruneErrorCb func(err error)
}

func (l *SelectiveListener) OnPrune(
	oldestBlockKept uint64,
	blocksPruned uint64,
	took time.Duration,
) {
	if l.OnPruneCb != nil {
		l.OnPruneCb(oldestBlockKept, blocksPruned, took)
	}
}

func (l *SelectiveListener) OnPruneError(err error) {
	if l.OnPruneErrorCb != nil {
		l.OnPruneErrorCb(err)
	}
}
