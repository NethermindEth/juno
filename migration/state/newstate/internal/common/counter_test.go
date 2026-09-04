package common

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestCounter_DoesNotLogBeforeTimeLogRate(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := log.NewZapLoggerWithCore(core)

	c := NewCounter(logger, time.Hour, "")
	c.Log(uint64(db.Megabyte), 5, 0)

	assert.Zero(t, recorded.Len())
}

func TestCounter_LogRoundsMBAndAttributesCaller(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := log.NewZapLoggerWithCore(core)

	c := NewCounter(logger, time.Millisecond, "")
	// Force elapsed > timeLogRate without sleeping.
	c.start = time.Now().Add(-time.Second)

	const bytes uint64 = 12_345_678 // arbitrary; not a clean MB count
	c.size = bytes
	c.completedAddrs = 4242

	c.Log(0, 0, 0)

	require.Equal(t, 1, recorded.Len())
	entry := recorded.All()[0]
	fields := entry.ContextMap()

	assert.Equal(t, "write speed", entry.Message)

	require.True(t, entry.Caller.Defined, "caller must be captured")
	assert.Equal(t, "counter_test.go", filepath.Base(entry.Caller.File),
		"log must be attributed to caller of Counter.Log, not counter.go itself")

	mb := fields["MB"].(float64)
	mbPerS := fields["MB/s"].(float64)
	elapsed := fields["time"].(float64)

	mbsExact := float64(bytes) / float64(db.Megabyte)
	assert.Equal(t, math.Round(mbsExact*100)/100, mb)
	assert.Equal(t, math.Round(mbsExact/elapsed*100)/100, mbPerS)
	assertTwoDecimalsOrFewer(t, "MB", mb)
	assertTwoDecimalsOrFewer(t, "MB/s", mbPerS)

	assert.Equal(t, uint64(4242), fields["completedContracts"])

	_, hasPhase := fields["phase"]
	assert.False(t, hasPhase, "empty phaseName must be omitted from the log entry")
}

func TestCounter_LogIncludesPhaseWhenSet(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := log.NewZapLoggerWithCore(core)

	c := NewCounter(logger, time.Millisecond, "class-hash")
	c.start = time.Now().Add(-time.Second)
	c.Log(0, 0, 0)

	require.Equal(t, 1, recorded.Len())
	fields := recorded.All()[0].ContextMap()
	assert.Equal(t, "class-hash", fields["phase"])
}

func assertTwoDecimalsOrFewer(t *testing.T, name string, v float64) {
	t.Helper()
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return
	}
	if got := len(s) - i - 1; got > 2 {
		t.Errorf("%s = %s has %d decimals, want at most 2", name, s, got)
	}
}
