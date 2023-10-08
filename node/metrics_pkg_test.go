package node

import (
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
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
