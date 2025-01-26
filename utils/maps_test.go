package utils

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortedMap(t *testing.T) {
	m := map[int]string{3: "three", 1: "one", 4: "four", 2: "two", 0: "zero"}

	keys := make([]int, 0, len(m))
	for k, v := range SortedMap(m) {
		keys = append(keys, k)
		assert.Equal(t, m[k], v)
	}
	assert.Len(t, keys, len(m))
	assert.True(t, slices.IsSorted(keys))
}
