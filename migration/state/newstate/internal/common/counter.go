package common

import (
	"fmt"
	"math"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// defaultUnitLabel names the work unit in the log line. Phases that walk
// contract addresses keep it; phases counting something else override it via
// [Counter.SetProgress].
const defaultUnitLabel = "completedContracts"

type Counter struct {
	logger      log.StructuredLogger
	timeLogRate time.Duration
	phaseName   string
	start       time.Time
	size        uint64
	units       uint64
	entryCount  uint64

	// Set by SetProgress. When allUnits is non-zero the log line also
	// reports how far the phase has come and how long it has been running.
	unit           string
	migrationStart time.Time
	allUnits       uint64
	allEntries     uint64
	totalUnits     uint64
	totalEntries   uint64
}

func NewCounter(logger log.StructuredLogger, timeLogRate time.Duration, phaseName string) Counter {
	if zl, ok := logger.(*log.ZapLogger); ok {
		logger = zl.WithOptions(zap.AddCallerSkip(1))
	}
	now := time.Now()
	return Counter{
		logger:         logger,
		timeLogRate:    timeLogRate,
		phaseName:      phaseName,
		start:          now,
		migrationStart: now,
		unit:           defaultUnitLabel,
	}
}

// SetProgress names the work unit and declares how much of it there is, which
// turns on the percentage and total-runtime fields. Call it before the first
// Log; allUnits of zero leaves the extra fields off.
func (c *Counter) SetProgress(unit string, allUnits, allEntries uint64) {
	if unit != "" {
		c.unit = unit
	}
	c.allUnits = allUnits
	c.allEntries = allEntries
}

func (c *Counter) Log(byteSize uint64, completedUnits, entryCount int) {
	c.size += byteSize
	c.units += uint64(completedUnits)
	c.entryCount += uint64(entryCount)
	c.totalUnits += uint64(completedUnits)
	c.totalEntries += uint64(entryCount)

	const cent = 100

	now := time.Now()
	elapsed := now.Sub(c.start).Seconds()
	if elapsed <= c.timeLogRate.Seconds() {
		return
	}

	round := func(v float64) float64 { return math.Round(v*cent) / cent }

	mbs := float64(c.size) / float64(db.Megabyte)
	fields := make([]zap.Field, 0, 11)
	if c.phaseName != "" {
		fields = append(fields, zap.String("phase", c.phaseName))
	}
	fields = append(fields,
		zap.Float64("MB", round(mbs)),
		zap.Float64("MB/s", round(mbs/elapsed)),
		zap.Uint64(c.unit, c.units),
		zap.Float64(c.unit+"/s", round(float64(c.units)/elapsed)),
		zap.Uint64("entries", c.entryCount),
		zap.Float64("entries/s", round(float64(c.entryCount)/elapsed)),
		zap.Float64("time", round(elapsed)),
	)
	if c.allUnits > 0 {
		fields = append(fields,
			zap.String(c.unit+"_processed", fmtPercent(c.totalUnits, c.allUnits)),
			zap.String("entries_processed", fmtPercent(c.totalEntries, c.allEntries)),
			zap.Float64("totalTime", round(now.Sub(c.migrationStart).Seconds())),
		)
	}
	c.logger.Info("write speed", fields...)

	c.start = now
	c.size = 0
	c.units = 0
	c.entryCount = 0
}

func fmtPercent(done, total uint64) string {
	if total == 0 {
		return "100.0%"
	}
	return fmt.Sprintf("%.1f%%", 100.0*float64(done)/float64(total))
}
