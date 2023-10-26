package db

import "time"

// PebbleMetrics is a structure that holds metrics related to a Pebble database's performance and resource usage.
type PebbleMetrics struct {
	CompTime       time.Duration // Time spent in the compaction.
	CompRead       uint64        // Amount of data read during compaction.
	CompWrite      uint64        // Amount of data written during compaction.
	WriteDelayN    uint64        // Number of write delays due to compaction.
	WriteDelay     time.Duration // Duration of write delays caused by compaction.
	DiskSize       uint64        // Total size of all levels in the database.
	DiskRead       uint64        // Amount of data read from the database.
	DiskWrite      uint64        // Amount of data written to the database.
	MemComps       uint32        // Number of memory compactions.
	Level0Comp     uint32        // Number of table compactions in level 0.
	NonLevel0Comp  uint32        // Number of table compactions in non-level 0.
	SeekComp       uint32        // Number of table compactions caused by read operations.
	LevelFiles     []uint64      // Amount of files in each level.
	ManualMemAlloc uint64        // Size of non-managed memory allocated.
}

// PebbleListener is an interface for listening to and handling PebbleMetrics data.
type PebbleListener interface {
	// OnPebbleMetrics is a method to handle PebbleMetrics data.
	OnPebbleMetrics(*PebbleMetrics)
}

type EventListener interface {
	OnIO(write bool)
	PebbleListener
}

type SelectiveListener struct {
	OnIOCb func(write bool)

	OnPebbleMetricsCb func(*PebbleMetrics)
}

func (l *SelectiveListener) OnIO(write bool) {
	if l.OnIOCb != nil {
		l.OnIOCb(write)
	}
}

// OnPebbleMetrics is a method that allows a SelectiveListener to handle PebbleMetrics data.
func (l *SelectiveListener) OnPebbleMetrics(m *PebbleMetrics) {
	if l.OnPebbleMetricsCb != nil {
		l.OnPebbleMetricsCb(m)
	}
}
