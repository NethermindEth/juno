package node

import (
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPebbleMetrics(t *testing.T) {
	listener := newPebbleListener()
	var (
		wg    sync.WaitGroup
		count = 3
	)
	wg.Add(1)
	selectiveListener := &db.SelectiveListener{
		OnPebbleMetricsCb: func(m *db.PebbleMetrics) {
			listener.gather(m)
			count--
			if count == 0 {
				wg.Done()
			}
		},
	}
	testDB := pebble.NewMemTest().WithListener(selectiveListener)
	testDB.Meter(1 * time.Second)
	wg.Wait()
}

func TestDualHistogram(t *testing.T) {
	src := prometheus.NewHistogram(prometheus.HistogramOpts{})
	wrapped := &dualHistogram{
		valueHist:      src,
		descrHistogram: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "foo"}),
	}
	prometheus.MustRegister(wrapped)
	src.Observe(1)
	// ensure that only foo is registered and src changes are seen.
	assert.Equal(t, 0, testutil.CollectAndCount(wrapped, "bar"))
	assert.Equal(t, 1, testutil.CollectAndCount(wrapped, "foo"))
}
