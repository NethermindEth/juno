package iter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPull(t *testing.T) {
	t.Run("Iterate over all values", func(t *testing.T) {
		it := newIterator(1, 2, 3, 4, 5)
		next, _ := Pull(it)

		for i := 1; i <= 5; i++ {
			v, ok := next()
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}

		v, ok := next()
		assert.Zero(t, v)
		assert.False(t, ok)
	})
	t.Run("Iterate and stop in the middle", func(t *testing.T) {
		it := newIterator(1, 2, 3, 4, 5)
		next, stop := Pull(it)

		for i := 1; i <= 3; i++ {
			v, ok := next()
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}
		stop()

		v, ok := next()
		assert.Zero(t, v)
		assert.False(t, ok)
	})
}

func newIterator[T any](values ...T) Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range values {
			if !yield(v) {
				break
			}
		}
	}
}
