package utils

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
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

func TestToPtrSlice(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var input []int
		assert.Nil(t, ToPtrSlice(input))
	})

	t.Run("pointers dereference to input values", func(t *testing.T) {
		input := []int{1, 2, 3}
		actual := ToPtrSlice(input)
		assert.Equal(t, []int{1, 2, 3}, []int{*actual[0], *actual[1], *actual[2]})
	})

	t.Run("pointers do not alias the input's backing array", func(t *testing.T) {
		input := []int{1, 2, 3}
		actual := ToPtrSlice(input)
		input[0] = 99
		assert.Equal(t, 1, *actual[0], "mutation of input should not affect copied pointers")
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

func TestNonNilSlice(t *testing.T) {
	t.Run("nil slice returns empty non-nil slice", func(t *testing.T) {
		var input []int
		actual := NonNilSlice(input)
		assert.NotNil(t, actual)
		assert.Empty(t, actual)
	})

	t.Run("empty slice is returned as-is", func(t *testing.T) {
		input := []int{}
		actual := NonNilSlice(input)
		assert.NotNil(t, actual)
		assert.Empty(t, actual)
	})

	t.Run("non-empty slice is returned unchanged", func(t *testing.T) {
		input := []int{1, 2, 3}
		actual := NonNilSlice(input)
		assert.Equal(t, []int{1, 2, 3}, actual)
	})
}

func TestDerefSlice(t *testing.T) {
	t.Run("nil pointer returns nil", func(t *testing.T) {
		var input *[]int
		actual := DerefSlice(input)
		assert.Nil(t, actual)
	})

	t.Run("pointer to empty slice returns empty slice", func(t *testing.T) {
		input := []int{}
		actual := DerefSlice(&input)
		assert.NotNil(t, actual)
		assert.Empty(t, actual)
	})

	t.Run("pointer to non-empty slice returns the underlying slice", func(t *testing.T) {
		input := []int{1, 2, 3}
		actual := DerefSlice(&input)
		assert.Equal(t, []int{1, 2, 3}, actual)
	})
}

func TestFeltArrToString(t *testing.T) {
	t.Run("empty slice returns empty string", func(t *testing.T) {
		actual := FeltArrToString([]*felt.Felt{})
		assert.Equal(t, "", actual)
	})

	t.Run("single element has no separator", func(t *testing.T) {
		input := []*felt.Felt{felt.NewFromUint64[felt.Felt](1)}
		actual := FeltArrToString(input)
		assert.Equal(t, "0x1", actual)
	})

	t.Run("multiple elements are comma-separated", func(t *testing.T) {
		input := []*felt.Felt{
			felt.NewFromUint64[felt.Felt](1),
			felt.NewFromUint64[felt.Felt](2),
			felt.NewFromUint64[felt.Felt](255),
		}
		actual := FeltArrToString(input)
		assert.Equal(t, "0x1, 0x2, 0xff", actual)
	})
}
