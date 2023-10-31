package sync

import "time"

type EventListener interface {
	OnSyncStepDone(op string, blockNum uint64, took time.Duration)
	OnReorg(blockNum uint64)
}

type SelectiveListener struct {
	OnSyncStepDoneCb func(op string, blockNum uint64, took time.Duration)
	OnReorgCb        func(blockNum uint64)
}

func (l *SelectiveListener) OnSyncStepDone(op string, blockNum uint64, took time.Duration) {
	if l.OnSyncStepDoneCb != nil {
		l.OnSyncStepDoneCb(op, blockNum, took)
	}
}

func (l *SelectiveListener) OnReorg(blockNum uint64) {
	if l.OnReorgCb != nil {
		l.OnReorgCb(blockNum)
	}
}
