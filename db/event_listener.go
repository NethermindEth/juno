package db

type LevelsMetrics struct {
	Level [7]struct {
		Sublevels       int32
		NumFiles        int64
		Size            int64
		Score           float64
		BytesIn         uint64
		BytesIngested   uint64
		BytesMoved      uint64
		BytesRead       uint64
		BytesCompacted  uint64
		BytesFlushed    uint64
		TablesCompacted uint64
		TablesFlushed   uint64
		TablesIngested  uint64
		TablesMoved     uint64
	}
}

type LevelsListener interface {
	OnLevels(LevelsMetrics)
}

type EventListener interface {
	OnIO(write bool)
	LevelsListener
}

type SelectiveListener struct {
	OnIOCb     func(write bool)
	OnLevelsCb func(LevelsMetrics)
}

func (l *SelectiveListener) OnIO(write bool) {
	if l.OnIOCb != nil {
		l.OnIOCb(write)
	}
}

func (l *SelectiveListener) OnLevels(m LevelsMetrics) {
	if l.OnLevelsCb != nil {
		l.OnLevelsCb(m)
	}
}
