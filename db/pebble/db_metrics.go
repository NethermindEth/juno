package pebble

import (
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

// onCompactionBegin is a callback for the start of a compaction process.
//
//nolint:gocritic
func (d *DB) onCompactionBegin(info pebble.CompactionInfo) {
	d.compStartTime = time.Now()
	d.activeComp++
	if info.Input[0].Level == 0 {
		d.level0Comp.Add(1)
	} else {
		d.nonLevel0Comp.Add(1)
	}
}

// onCompactionEnd is a callback for the end of a compaction process.
//
//nolint:gocritic
func (d *DB) onCompactionEnd(info pebble.CompactionInfo) {
	d.compTime.Add(int64(time.Since(d.compStartTime)))
	d.activeComp--
}

// onWriteStallBegin is a callback for the start of a write stall.\
func (d *DB) onWriteStallBegin(b pebble.WriteStallBeginInfo) {
	d.writeDelayStartTime = time.Now()
	d.writeDelayCount.Add(1)
}

// onWriteStallEnd is a callback for the end of a write stall.
func (d *DB) onWriteStallEnd() {
	d.writeDelayTime.Add(int64(time.Since(d.writeDelayStartTime)))
}

// meter continuously gathers metrics at the specified interval and reports them to the underlying listener.
func (d *DB) meter(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	// Gather metrics continuously at specified interval.
	for i := 1; ; i++ {
		var (
			compWrite uint64
			compRead  uint64

			stats           = d.pebble.Metrics()
			compTime        = time.Duration(d.compTime.Load())
			writeDelayCount = d.writeDelayCount.Load()
			writeDelayTime  = time.Duration(d.writeDelayTime.Load())
			nWrite          = stats.WAL.BytesWritten
		)
		for x := 0; x < len(stats.Levels); x++ {
			levelMetrics := stats.Levels[x]
			nWrite += levelMetrics.BytesCompacted
			nWrite += levelMetrics.BytesFlushed
			compWrite += levelMetrics.BytesCompacted
			compRead += levelMetrics.BytesRead
		}

		metrics := db.PebbleMetrics{
			CompTime:      compTime,
			CompRead:      compRead,
			CompWrite:     compWrite,
			WriteDelayN:   writeDelayCount,
			WriteDelay:    writeDelayTime,
			DiskSize:      stats.DiskSpaceUsage(),
			DiskRead:      0, // pebble doesn't track non-compaction reads
			DiskWrite:     nWrite,
			MemComps:      uint32(stats.Flush.Count),
			Level0Comp:    d.level0Comp.Load(),
			NonLevel0Comp: d.nonLevel0Comp.Load(),
			SeekComp:      uint32(stats.Compact.ReadCount),
			LevelFiles:    make([]uint64, len(stats.Levels)),
			// See https://github.com/cockroachdb/pebble/pull/1628#pullrequestreview-1026664054
			ManualMemAlloc: uint64(
				stats.BlockCache.Size + int64(
					stats.MemTable.Size,
				) + int64(
					stats.MemTable.ZombieSize,
				),
			),
		}
		for i := 0; i < len(stats.Levels); i++ {
			metrics.LevelFiles[i] = uint64(stats.Levels[i].NumFiles)
		}

		// Notify the listener.
		d.listener.OnPebbleMetrics(&metrics)

		<-timer.C
		timer.Reset(interval)
	}
}
