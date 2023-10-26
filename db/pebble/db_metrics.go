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

	// Create storage for delta calculation.
	var (
		compTimes        [2]time.Duration
		writeDelayCounts [2]uint64
		writeDelayTimes  [2]time.Duration
		compWrites       [2]uint64
		compReads        [2]uint64
		nWrites          [2]uint64
		memComps         [2]uint32
		lvl0Comps        [2]uint32
		nonLvl0Comps     [2]uint32
		seekComps        [2]uint32
	)

	// Gather metrics continuously at specified interval.
	for i := 1; ; i++ {
		var (
			current, previous = i % 2, (i - 1) % 2
			compWrite         uint64
			compRead          uint64

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
		compTimes[current] = compTime
		writeDelayCounts[current] = writeDelayCount
		writeDelayTimes[current] = writeDelayTime
		compWrites[current] = compWrite
		compReads[current] = compRead
		nWrites[current] = nWrite
		memComps[current] = uint32(stats.Flush.Count)
		lvl0Comps[current] = d.level0Comp.Load()
		nonLvl0Comps[current] = d.nonLevel0Comp.Load()
		seekComps[current] = uint32(stats.Compact.ReadCount)

		metrics := db.PebbleMetrics{
			CompTime:      compTimes[current] - compTimes[previous],
			CompRead:      compReads[current] - compReads[previous],
			CompWrite:     compWrites[current] - compWrites[previous],
			WriteDelayN:   writeDelayCounts[current] - writeDelayCounts[previous],
			WriteDelay:    writeDelayTimes[current] - writeDelayTimes[previous],
			DiskSize:      stats.DiskSpaceUsage(),
			DiskRead:      0, // pebble doesn't track non-compaction reads
			DiskWrite:     nWrites[current] - nWrites[previous],
			MemComps:      memComps[current] - memComps[previous],
			Level0Comp:    lvl0Comps[current] - lvl0Comps[previous],
			NonLevel0Comp: nonLvl0Comps[current] - nonLvl0Comps[previous],
			SeekComp:      seekComps[current] - seekComps[previous],
			LevelFiles:    make([]uint64, len(stats.Levels)),
			// See https://github.com/cockroachdb/pebble/pull/1628#pullrequestreview-1026664054
			ManualMemAlloc: uint64(stats.BlockCache.Size + int64(stats.MemTable.Size) + int64(stats.MemTable.ZombieSize)),
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
