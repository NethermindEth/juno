package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorityQueue(t *testing.T) {
	t.Run("Ok", func(t *testing.T) {
		low := make(chan int, 1)
		lowValue := 66
		low <- lowValue

		high := make(chan int, 1)
		highValue := 77
		high <- highValue

		// check that high values comes first
		queue := PriorityQueue(high, low)
		assert.Equal(t, highValue, <-queue)
		assert.Equal(t, lowValue, <-queue)

		// check that queue is closed
		close(low)
		close(high)

		_, ok := <-queue
		assert.False(t, ok)
	})
	t.Run("High is closed", func(t *testing.T) {
		high := make(chan int)
		close(high)

		const n = 10
		low := make(chan int, n)
		for i := 0; i < n; i++ {
			low <- i
		}
		close(low)

		// check that all values from low present in queue
		queue := PriorityQueue(high, low)
		for i := 0; i < n; i++ {
			v, ok := <-queue
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}

		// check that queue is closed
		_, ok := <-queue
		assert.False(t, ok)
	})
	t.Run("Low is closed", func(t *testing.T) {
		low := make(chan int)
		close(low)

		const n = 10
		high := make(chan int, n)
		for i := 0; i < n; i++ {
			high <- i
		}
		close(high)

		// check that all values from high present in queue
		queue := PriorityQueue(high, low)
		for i := 0; i < n; i++ {
			v, ok := <-queue
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}

		// check that queue is closed
		_, ok := <-queue
		assert.False(t, ok)
	})
}

func TestPipeline(t *testing.T) {
	t.Run("nil channel", func(t *testing.T) {
		outValue := Pipeline(nil, strconv.Itoa)
		done := PipelineEnd(outValue, func(string) {
			// this function should not be called
			t.Fail()
		})

		_, open := <-done
		assert.False(t, open)
	})
}
