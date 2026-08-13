package cborlite_test

import (
	"sync"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
)

// Eight types, so the goroutines below build eight plans at once rather than queueing on
// one. Nothing else names them, so each is built for the first time inside the test.
type (
	raced1 struct{ Value uint64 }
	raced2 struct{ Value uint64 }
	raced3 struct{ Value uint64 }
	raced4 struct{ Value uint64 }
	raced5 struct{ Value uint64 }
	raced6 struct{ Value uint64 }
	raced7 struct{ Value uint64 }
	raced8 struct{ Value uint64 }
)

// TestConcurrentFirstBuildsAreSafe guards the plan caches, which are plain maps behind
// buildMutex rather than sync.Map. Run under -race, this reports a data race, or panics
// with a concurrent map write, if a build path ever stops taking the lock.
func TestConcurrentFirstBuildsAreSafe(t *testing.T) {
	data := cborMap(cborText("Value"), []byte{0x07})

	// Half strict, so the strict caches are built concurrently too.
	decodes := []func() error{
		func() error { var v raced1; return cborlite.Unmarshal(data, &v) },
		func() error { var v raced2; return cborlite.Unmarshal(data, &v) },
		func() error { var v raced3; return cborlite.Unmarshal(data, &v) },
		func() error { var v raced4; return cborlite.Unmarshal(data, &v) },
		func() error { var v raced5; return cborlite.UnmarshalStrict(data, &v) },
		func() error { var v raced6; return cborlite.UnmarshalStrict(data, &v) },
		func() error { var v raced7; return cborlite.UnmarshalStrict(data, &v) },
		func() error { var v raced8; return cborlite.UnmarshalStrict(data, &v) },
	}

	var group sync.WaitGroup
	for range 4 {
		for _, decode := range decodes {
			group.Add(1)
			go func() {
				defer group.Done()
				assert.NoError(t, decode())
			}()
		}
	}
	group.Wait()
}
