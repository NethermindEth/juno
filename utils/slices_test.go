package utils

import (
	"errors"
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

func TestMapWithErrors(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		seq := MapWithErrors(input, func(i int) (string, error) {
			return strconv.Itoa(i), nil
		})

		count := 0
		seq(func(_ string, _ error) bool {
			count++
			return true
		})
		assert.Equal(t, 0, count)
	})

	t.Run("slice with all successful conversions", func(t *testing.T) {
		input := []int{1, 2, 3}
		expected := []string{"1", "2", "3"}

		var results []string
		seq := MapWithErrors(input, func(i int) (string, error) {
			return strconv.Itoa(i), nil
		})

		seq(func(s string, err error) bool {
			require.NoError(t, err)
			results = append(results, s)
			return true
		})

		assert.Equal(t, expected, results)
	})

	t.Run("slice with an error element", func(t *testing.T) {
		input := []int{1, 0, 2}
		expected := []int{1}
		var results []int
		var capturedErr error

		seq := MapWithErrors(input, func(i int) (int, error) {
			if i == 0 {
				return 0, errors.New("zero is not allowed")
			}
			return i, nil
		})

		seq(func(val int, err error) bool {
			if err != nil {
				capturedErr = err
				return false // stop iteration on error
			}
			results = append(results, val)
			return true
		})

		assert.Equal(t, expected, results)
		assert.EqualError(t, capturedErr, "zero is not allowed")
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

func TestSet(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		actual := Set(input)
		assert.Nil(t, actual)
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []int{}
		actual := Set(input)
		assert.Empty(t, actual)
	})

	t.Run("slice with no duplicates", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		actual := Set(input)
		assert.Equal(t, []int{1, 2, 3, 4, 5}, actual)
	})

	t.Run("slice with duplicates", func(t *testing.T) {
		input := []int{1, 2, 2, 3, 3, 3, 4, 5, 5}
		actual := Set(input)
		assert.Equal(t, []int{1, 2, 3, 4, 5}, actual)
	})

	t.Run("slice of strings with duplicates", func(t *testing.T) {
		input := []string{"a", "b", "b", "c", "c", "c"}
		actual := Set(input)
		assert.Equal(t, []string{"a", "b", "c"}, actual)
	})
}
