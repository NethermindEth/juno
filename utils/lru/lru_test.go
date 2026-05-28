package lru

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:dupl // duplicate tests as there's identical APIs
func TestNew(t *testing.T) {
	t.Run("returns usable cache for positive size", func(t *testing.T) {
		c := New[string, int](2)
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
			New[string, int](0)
		})
	})

	t.Run("panics on negative size", func(t *testing.T) {
		assert.PanicsWithError(t, "lru: must provide a positive size (size=-1)", func() {
			New[string, int](-1)
		})
	})
}

func TestCache_Remove(t *testing.T) {
	t.Run("removes present key", func(t *testing.T) {
		c := New[string, int](2)
		c.Add("a", 1)
		c.Add("b", 2)

		assert.True(t, c.Remove("a"))
		assert.Equal(t, 1, c.Len())

		_, ok := c.Get("a")
		assert.False(t, ok)
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		c := New[string, int](2)
		c.Add("a", 1)

		assert.False(t, c.Remove("missing"))
		assert.Equal(t, 1, c.Len())
	})
}

func TestCache_Purge(t *testing.T) {
	c := New[string, int](3)
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
func TestNewSimple(t *testing.T) {
	t.Run("returns usable cache for positive size", func(t *testing.T) {
		c := NewSimple[string, int](2)
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
			NewSimple[string, int](0)
		})
	})

	t.Run("panics on negative size", func(t *testing.T) {
		assert.PanicsWithError(t, "simplelru: must provide a positive size (size=-1)", func() {
			NewSimple[string, int](-1)
		})
	})
}

func TestSimpleCache_Purge(t *testing.T) {
	c := NewSimple[string, int](3)
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
