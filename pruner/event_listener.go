package pruner

import "time"

type EventListener interface {
	OnPrune(oldestBlockKept uint64, blocksPruned uint64, took time.Duration)
	OnPruneError(err error)
	OnL1Stale()
}

type SelectiveListener struct {
	OnPruneCb      func(oldestBlockKept uint64, blocksPruned uint64, took time.Duration)
	OnPruneErrorCb func(err error)
	OnL1StaleCb    func()
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

func (l *SelectiveListener) OnL1Stale() {
	if l.OnL1StaleCb != nil {
		l.OnL1StaleCb()
	}
}
