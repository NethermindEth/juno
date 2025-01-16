package utils

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedSet(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		set := NewOrderedSet[string, int]()

		// Test initial state
		assert.Equal(t, 0, set.Size())
		assert.Empty(t, set.List())
		assert.Empty(t, set.Keys())

		// Test Put and Get
		set.Put("a", 1)
		set.Put("b", 2)
		set.Put("c", 3)

		val, exists := set.Get("b")
		assert.True(t, exists)
		assert.Equal(t, 2, val)

		// Test size
		assert.Equal(t, 3, set.Size())

		// Test order preservation
		assert.Equal(t, []int{1, 2, 3}, set.List())
		assert.Equal(t, []string{"a", "b", "c"}, set.Keys())
	})

	t.Run("updating existing keys", func(t *testing.T) {
		set := NewOrderedSet[string, int]()

		set.Put("a", 1)
		set.Put("b", 2)
		set.Put("a", 10) // Update existing key

		// Check value was updated but order preserved
		assert.Equal(t, []int{10, 2}, set.List())
		assert.Equal(t, []string{"a", "b"}, set.Keys())

		val, exists := set.Get("a")
		assert.True(t, exists)
		assert.Equal(t, 10, val)
	})

	t.Run("non-existent keys", func(t *testing.T) {
		set := NewOrderedSet[string, int]()

		val, exists := set.Get("nonexistent")
		assert.False(t, exists)
		assert.Zero(t, val)
	})

	t.Run("concurrent access", func(t *testing.T) {
		set := NewOrderedSet[int, string]()
		var wg sync.WaitGroup

		// Concurrent writes
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				set.Put(n, string(rune(n)))
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = set.Size()
				_ = set.List()
				_ = set.Keys()
			}()
		}

		wg.Wait()
		assert.Equal(t, 100, set.Size())
	})

	t.Run("different types", func(t *testing.T) {
		// Test with different key-value type combinations
		set1 := NewOrderedSet[int, string]()
		set1.Put(1, "one")
		set1.Put(2, "two")
		assert.Equal(t, []string{"one", "two"}, set1.List())
		assert.Equal(t, []int{1, 2}, set1.Keys())

		set2 := NewOrderedSet[string, bool]()
		set2.Put("true", true)
		set2.Put("false", false)
		assert.Equal(t, []bool{true, false}, set2.List())
		assert.Equal(t, []string{"true", "false"}, set2.Keys())
	})

	t.Run("large number of elements", func(t *testing.T) {
		set := NewOrderedSet[int, int]()
		n := 10000

		// Insert elements
		for i := 0; i < n; i++ {
			set.Put(i, i*2)
		}

		assert.Equal(t, n, set.Size())

		// Verify all elements
		for i := 0; i < n; i++ {
			val, exists := set.Get(i)
			assert.True(t, exists)
			assert.Equal(t, i*2, val)
		}

		// Verify order
		keys := set.Keys()
		values := set.List()
		for i := 0; i < n; i++ {
			assert.Equal(t, i, keys[i])
			assert.Equal(t, i*2, values[i])
		}
	})

	t.Run("zero values", func(t *testing.T) {
		set := NewOrderedSet[string, int]()

		// Put zero values
		set.Put("zero", 0)
		set.Put("empty", 0)

		assert.Equal(t, 2, set.Size())
		val, exists := set.Get("zero")
		assert.True(t, exists)
		assert.Zero(t, val)
	})
}
