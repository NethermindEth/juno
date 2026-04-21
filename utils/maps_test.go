package utils

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToMap(t *testing.T) {
	t.Run("nil slice returns empty map", func(t *testing.T) {
		var input []int
		actual := ToMap(input, func(v int) (int, string) { return v, strconv.Itoa(v) })
		assert.NotNil(t, actual)
		assert.Empty(t, actual)
	})

	t.Run("empty slice returns empty map", func(t *testing.T) {
		input := []int{}
		actual := ToMap(input, func(v int) (int, string) { return v, strconv.Itoa(v) })
		assert.NotNil(t, actual)
		assert.Empty(t, actual)
	})

	t.Run("slice is mapped to key/value pairs", func(t *testing.T) {
		input := []int{1, 2, 3}
		actual := ToMap(input, func(v int) (int, string) { return v, strconv.Itoa(v) })
		expected := map[int]string{1: "1", 2: "2", 3: "3"}
		assert.Equal(t, expected, actual)
	})

	t.Run("duplicate keys keep the last value", func(t *testing.T) {
		type entry struct {
			k string
			v int
		}
		input := []entry{{"a", 1}, {"b", 2}, {"a", 3}}
		actual := ToMap(input, func(e entry) (string, int) { return e.k, e.v })
		assert.Equal(t, map[string]int{"a": 3, "b": 2}, actual)
	})
}

func TestToSlice(t *testing.T) {
	t.Run("nil map returns nil", func(t *testing.T) {
		var input map[int]string
		actual := ToSlice(input, func(k int, v string) string { return v })
		assert.Nil(t, actual)
	})

	t.Run("empty map returns empty slice", func(t *testing.T) {
		input := map[int]string{}
		actual := ToSlice(input, func(k int, v string) string { return v })
		assert.NotNil(t, actual)
		assert.Empty(t, actual)
	})

	t.Run("map is transformed into slice", func(t *testing.T) {
		input := map[int]string{1: "one", 2: "two", 3: "three"}
		actual := ToSlice(input, func(k int, v string) string {
			return strconv.Itoa(k) + ":" + v
		})
		sort.Strings(actual)
		assert.Equal(t, []string{"1:one", "2:two", "3:three"}, actual)
	})
}
