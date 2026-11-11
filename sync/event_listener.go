package sync

import "time"

type EventListener interface {
	OnSyncStepDone(op string, blockNum uint64, took time.Duration)
	OnReorg(blockNum uint64)
	// OnFetchStarted and OnFetchDone mark the start and end of each block fetch.
	OnFetchStarted()
	OnFetchDone()
	// OnBlockStored reports a block that sync stored, with its transaction count.
	OnBlockStored(blockNum uint64, txCount int)
}

type SelectiveListener struct {
	OnSyncStepDoneCb func(op string, blockNum uint64, took time.Duration)
	OnReorgCb        func(blockNum uint64)
	OnFetchStartedCb func()
	OnFetchDoneCb    func()
	OnBlockStoredCb  func(blockNum uint64, txCount int)
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

func (l *SelectiveListener) OnFetchStarted() {
	if l.OnFetchStartedCb != nil {
		l.OnFetchStartedCb()
	}
}

func (l *SelectiveListener) OnFetchDone() {
	if l.OnFetchDoneCb != nil {
		l.OnFetchDoneCb()
	}
}

func (l *SelectiveListener) OnBlockStored(blockNum uint64, txCount int) {
	if l.OnBlockStoredCb != nil {
		l.OnBlockStoredCb(blockNum, txCount)
	}
}
