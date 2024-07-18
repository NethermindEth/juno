package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		actual := Map(input, strconv.Itoa)
		assert.Nil(t, actual)
	})
	t.Run("slice with data", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5, 6}
		expected := []string{"1", "2", "3", "4", "5", "6"}

		strings := Map(input, strconv.Itoa)
		assert.Equal(t, expected, strings)
	})
}

func TestFilter(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		actual := Filter(input, func(int) bool { return false })
		assert.Nil(t, actual)
	})
	t.Run("filter some elements", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5, 6}
		actual := Filter(input, func(v int) bool { return v%2 == 0 })
		assert.Equal(t, []int{2, 4, 6}, actual)
	})
}

func TestAll(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		allValue := All(input, func(int) bool {
			return false
		})
		assert.True(t, allValue)
	})
	t.Run("no element matches the predicate", func(t *testing.T) {
		input := []int{1, 2, 3, 4}
		allEven := All(input, func(v int) bool {
			return v%2 == 0
		})
		assert.False(t, allEven)
	})
	t.Run("all elements match the predicate", func(t *testing.T) {
		input := []int{1, 3, 5, 7}
		allOdd := All(input, func(v int) bool {
			return v%2 != 0
		})
		assert.True(t, allOdd)
	})
}

func TestAnyOf(t *testing.T) {
	t.Run("nil args", func(t *testing.T) {
		var input []int
		assert.False(t, AnyOf(0, input...))
	})

	t.Run("element is in args", func(t *testing.T) {
		assert.True(t, AnyOf("2", "1", "2", "3", "4", "5", "6"))
	})

	t.Run("element is not in args", func(t *testing.T) {
		assert.False(t, AnyOf("9", "1", "2", "3", "4", "5", "6"))
	})
}

func FuzzFilter(f *testing.F) {
	f.Fuzz(func(t *testing.T, bytes []byte) {
		var expected []byte
		for _, b := range bytes {
			if b&1 == 0 {
				expected = append(expected, b)
			}
		}
		actual := Filter(bytes, func(x byte) bool {
			return x&1 == 0
		})
		assert.Equal(t, expected, actual)
	})
}
