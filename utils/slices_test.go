package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestUnique(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		actual := Unique(input)
		assert.Nil(t, actual)
	})
	t.Run("empty slice returns nil", func(t *testing.T) {
		input := []int{}
		actual := Unique(input)
		assert.Nil(t, actual)
	})
	t.Run("slice with data", func(t *testing.T) {
		expected := []int{1, 2, 3}
		input := expected
		input = append(input, expected...)
		actual := Unique(input)
		assert.Equal(t, expected, actual)
	})
	t.Run("panic when called on pointers", func(t *testing.T) {
		defer func() {
			r := recover()
			assert.NotNil(t, r)
			assert.Contains(t, r.(string), "Unique() cannot be used with a slice of pointers")
		}()
		input := []*int{new(int), new(int)}
		Unique(input)
	})
	t.Run("with key function", func(t *testing.T) {
		type thing struct {
			id   int
			name string
		}

		things := []thing{
			{1, "one"}, {1, "two"}, {2, "one"}, {2, "two"},
		}

		require.Len(t, UniqueFunc(things, func(t thing) int { return t.id }), 2)
		require.Len(t, UniqueFunc(things, func(t thing) string { return t.name }), 2)
		require.Equal(t, things, UniqueFunc(things, func(t thing) thing { return t }))
	})
}
