package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlatten(t *testing.T) {
	expected := []int{1, 2, 3, 4, 5, 6}
	halfLen := len(expected) / 2
	actual := Flatten(expected[:halfLen], expected[halfLen:])
	assert.Equal(t, expected, actual)
}

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

func TestIndexFunc(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		idx := IndexFunc(input, func(int) bool {
			return true
		})
		assert.Equal(t, -1, idx)
	})
	t.Run("element not present", func(t *testing.T) {
		input := []int{1, 2, 3, 4}
		idx := IndexFunc(input, func(v int) bool {
			return v < 0
		})
		assert.Equal(t, -1, idx)
	})
	t.Run("element present", func(t *testing.T) {
		input := []int{1, 2, 3, 4}
		idx := IndexFunc(input, func(v int) bool {
			return v == 3
		})
		assert.Equal(t, 2, idx)
	})
}
