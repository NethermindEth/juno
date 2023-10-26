package node

import (
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPebbleMetrics tests the collection of Pebble database metrics and ensures that
// no panics occur during metric collection. It also verifies that all metric fields are
// properly collected and match the expected values.
func TestPebbleMetrics(t *testing.T) {
	var (
		listener = newPebbleListener(prometheus.NewRegistry())
		wg       sync.WaitGroup
		gathered bool
		metrics  *db.PebbleMetrics
	)
	wg.Add(1)
	selectiveListener := &db.SelectiveListener{
		OnPebbleMetricsCb: func(m *db.PebbleMetrics) {
			if gathered {
				return
			}
			listener.gather(m)
			metrics = m
			gathered = true
			wg.Done()
		},
	}
	testDB, err := pebble.New(t.TempDir(), utils.NewNopZapLogger())
	require.NoError(t, err)
	defer testDB.Close()
	testDB = testDB.WithListener(selectiveListener)
	// do some arbitrary load in order to have some data in gathered metrics
	for i := 0; i < 2<<10; i++ {
		txn := testDB.NewTransaction(true)
		for x := 0; x < 2<<5; x++ {
			key := make([]byte, 32)
			_, err := rand.Read(key)
			require.NoError(t, err)
			require.NoError(t, txn.Set(key, key))
		}
		require.NoError(t, txn.Commit())
	}

	testDB.Meter(1 * time.Second)
	wg.Wait()

	assert.Equal(t, float64(metrics.CompRead), testutil.ToFloat64(listener.compRead))
	assert.Equal(t, float64(metrics.CompWrite), testutil.ToFloat64(listener.compWrite))
	assert.Equal(t, metrics.CompTime.Seconds(), testutil.ToFloat64(listener.compTime))
	assert.Equal(t, float64(metrics.WriteDelayN), testutil.ToFloat64(listener.writeDelays))
	assert.Equal(t, metrics.WriteDelay.Seconds(), testutil.ToFloat64(listener.writeDelay))
	assert.Equal(t, float64(metrics.DiskSize), testutil.ToFloat64(listener.diskSize))
	assert.Equal(t, float64(metrics.DiskRead), testutil.ToFloat64(listener.diskRead))
	assert.Equal(t, float64(metrics.DiskWrite), testutil.ToFloat64(listener.diskWrite))
	assert.Equal(t, float64(metrics.MemComps), testutil.ToFloat64(listener.memComp))
	assert.Equal(t, float64(metrics.Level0Comp), testutil.ToFloat64(listener.level0Comp))
	assert.Equal(t, float64(metrics.NonLevel0Comp), testutil.ToFloat64(listener.nonLevel0Comp))
	assert.Equal(t, float64(metrics.SeekComp), testutil.ToFloat64(listener.seekComp))
	assert.Equal(t, float64(metrics.ManualMemAlloc), testutil.ToFloat64(listener.manualMemAlloc))
}
