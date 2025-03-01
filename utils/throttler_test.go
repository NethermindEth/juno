package utils_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrottler(t *testing.T) {
	throttledRes := utils.NewThrottler(2, new(int)).WithMaxQueueLen(2)
	waitOn := make(chan struct{})

	var runCount int64
	doer := func(ptr *int) error {
		if ptr == nil {
			return errors.New("nilptr")
		}
		<-waitOn
		atomic.AddInt64(&runCount, 1)
		return nil
	}

	var wg sync.WaitGroup
	do := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, throttledRes.Do(doer))
		}()
		time.Sleep(time.Millisecond)
	}

	do()
	assert.Equal(t, 0, throttledRes.QueueLen())
	do()
	assert.Equal(t, 0, throttledRes.QueueLen())

	do() // should be queued
	assert.Equal(t, 1, throttledRes.QueueLen())
	do() // should be queued
	assert.Equal(t, 2, throttledRes.QueueLen())

	require.ErrorIs(t, throttledRes.Do(doer), utils.ErrResourceBusy)

	waitOn <- struct{}{} // release one of the slots
	time.Sleep(time.Millisecond)
	assert.Equal(t, 1, throttledRes.QueueLen())
	waitOn <- struct{}{} // release another slot, queue should be empty
	time.Sleep(time.Millisecond)
	assert.Equal(t, 0, throttledRes.QueueLen())

	// release the jobs waiting
	waitOn <- struct{}{}
	waitOn <- struct{}{}
	wg.Wait()
	assert.Equal(t, int64(4), runCount)
}
