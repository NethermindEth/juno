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
	waitOn <- struct{}{} // release another slot, qeueue should be empty
	time.Sleep(time.Millisecond)
	assert.Equal(t, 0, throttledRes.QueueLen())

	// release the jobs waiting
	waitOn <- struct{}{}
	waitOn <- struct{}{}
	wg.Wait()
	assert.Equal(t, int64(4), runCount)
}

func FuzzThrottler(f *testing.F) {
	f.Fuzz(func(t *testing.T, concurrencyBudget uint, resource int) {
		throttledRes := utils.NewThrottler(concurrencyBudget, &resource)
		wg := &sync.WaitGroup{}
		waitOn := make(chan struct{})
		var ranCount uint32

		doer := func(ptr *int) error {
			if ptr == nil {
				return errors.New("nilptr")
			}
			<-waitOn
			atomic.AddUint32(&ranCount, 1)
			return nil
		}

		do := func() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				require.NoError(t, throttledRes.Do(doer))
			}()
			time.Sleep(time.Millisecond)
		}

		for i := 0; i < int(concurrencyBudget); i++ {
			do()
		}
		assert.Equal(t, 0, throttledRes.QueueLen(), "queue should be empty")
		do()
		assert.Equal(t, 1, throttledRes.QueueLen(), "queue should be 1")

		for i := 0; i < int(concurrencyBudget+1); i++ {
			waitOn <- struct{}{}
		}

		wg.Wait()
		assert.Equal(t, 0, throttledRes.QueueLen(), "queue should be empty")
		assert.Equal(t, int(concurrencyBudget)+1, int(atomic.LoadUint32(&ranCount)), "ranCount should be concurrencyBudget+1")
	})
}
