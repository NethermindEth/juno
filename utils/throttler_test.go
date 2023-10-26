package utils_test

import (
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrottler(t *testing.T) {
	throttledRes := utils.NewThrottler(2, new(int)).WithMaxQueueLen(2)
	waitOn := make(chan struct{})
	ranCount := 0

	doer := func(ptr *int) error {
		if ptr == nil {
			return errors.New("nilptr")
		}
		<-waitOn
		ranCount++
		return nil
	}
	do := func() {
		go func() {
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
	time.Sleep(time.Millisecond)
	assert.Equal(t, 4, ranCount)
}
