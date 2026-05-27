package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:dupl // duplicate tests as there's identical APIs
func TestNewLRU(t *testing.T) {
	t.Run("returns usable cache for positive size", func(t *testing.T) {
		c := NewLRU[string, int](2)
		require.NotNil(t, c)
		assert.Equal(t, 0, c.Len())

		c.Add("a", 1)
		c.Add("b", 2)
		assert.Equal(t, 2, c.Len())

		v, ok := c.Get("a")
		assert.True(t, ok)
		assert.Equal(t, 1, v)
	})

	t.Run("panics on zero size", func(t *testing.T) {
		assert.PanicsWithError(t, "lru: must provide a positive size (size=0)", func() {
			NewLRU[string, int](0)
		})
	})

	t.Run("panics on negative size", func(t *testing.T) {
		assert.PanicsWithError(t, "lru: must provide a positive size (size=-1)", func() {
			NewLRU[string, int](-1)
		})
	})
}

func TestLRU_Remove(t *testing.T) {
	t.Run("removes present key", func(t *testing.T) {
		c := NewLRU[string, int](2)
		c.Add("a", 1)
		c.Add("b", 2)

		assert.True(t, c.Remove("a"))
		assert.Equal(t, 1, c.Len())

		_, ok := c.Get("a")
		assert.False(t, ok)
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		c := NewLRU[string, int](2)
		c.Add("a", 1)

		assert.False(t, c.Remove("missing"))
		assert.Equal(t, 1, c.Len())
	})
}

func TestLRU_Purge(t *testing.T) {
	c := NewLRU[string, int](3)
	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3)

	c.Purge()
	assert.Equal(t, 0, c.Len())

	_, ok := c.Get("a")
	assert.False(t, ok)

	c.Add("d", 4)
	assert.Equal(t, 1, c.Len())
}

//nolint:dupl // duplicate tests as there's identical APIs
func TestNewSimpleLRU(t *testing.T) {
	t.Run("returns usable cache for positive size", func(t *testing.T) {
		c := NewSimpleLRU[string, int](2)
		require.NotNil(t, c)
		assert.Equal(t, 0, c.Len())

		c.Add("a", 1)
		c.Add("b", 2)
		assert.Equal(t, 2, c.Len())

		v, ok := c.Get("a")
		assert.True(t, ok)
		assert.Equal(t, 1, v)
	})

	t.Run("panics on zero size", func(t *testing.T) {
		assert.PanicsWithError(t, "simplelru: must provide a positive size (size=0)", func() {
			NewSimpleLRU[string, int](0)
		})
	})

	t.Run("panics on negative size", func(t *testing.T) {
		assert.PanicsWithError(t, "simplelru: must provide a positive size (size=-1)", func() {
			NewSimpleLRU[string, int](-1)
		})
	})
}

func TestSimpleLRU_Purge(t *testing.T) {
	c := NewSimpleLRU[string, int](3)
	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3)

	c.Purge()
	assert.Equal(t, 0, c.Len())

	_, ok := c.Get("a")
	assert.False(t, ok)

	c.Add("d", 4)
	assert.Equal(t, 1, c.Len())
}
