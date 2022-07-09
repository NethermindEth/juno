package cache

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNewLRUCache(t *testing.T) {
	c := NewLRUCache(10)
	if c.Capacity() != 10 {
		t.Errorf("Expected capacity to be 10, got %d", c.Capacity())
	}
	if c.Count() != 0 {
		t.Errorf("Expected count to be 0, got %d", c.Count())
	}
}

func TestLRUCache_Put(t *testing.T) {
	c := NewLRUCache(10)
	// Fill the cache
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		c.Put(key, value)
		if c.Count() != i+1 {
			t.Errorf("Expected count to be %d, got %d", i+1, c.Count())
		}
	}
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%d", 10+i))
		value := []byte(fmt.Sprintf("value%d", 10+i))
		c.Put(key, value)
		if c.Count() != 10 {
			t.Errorf("Expected count to be 10, got %d", c.Count())
		}
		deletedKey := []byte(fmt.Sprintf("key%d", i))
		if v := c.Get(deletedKey); v != nil {
			t.Errorf("Expected value to be nil, got %s", v)
		}
	}
}

func TestLRUCache_Get(t *testing.T) {
	c := NewLRUCache(10)
	for i := 0; i < 3; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		c.Put(key, value)
	}
	for _, i := range []int{2, 1, 0} {
		key := []byte(fmt.Sprintf("key%d", i))
		want := []byte(fmt.Sprintf("value%d", i))
		value := c.Get(key)
		if !bytes.Equal(value, want) {
			t.Errorf("Expected value to be %s, got %s", want, value)
		}
	}
	if value := c.Get([]byte("key4")); value != nil {
		t.Errorf("Expected value to be nil, got %s", value)
	}
}

func TestLRUCache_Count(t *testing.T) {
	c := NewLRUCache(10)
	if c.Count() != 0 {
		t.Errorf("Expected count to be 0, got %d", c.Count())
	}
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		c.Put(key, value)
		if c.Count() != i+1 {
			t.Errorf("Expected count to be %d, got %d", i+1, c.Count())
		}
	}
}

func TestLRUCache_Capacity(t *testing.T) {
	c := NewLRUCache(100)
	if c.Capacity() != 100 {
		t.Errorf("Expected capacity to be 100, got %d", c.Capacity())
	}
}

func TestLRUCache_Clear(t *testing.T) {
	c := NewLRUCache(10)
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		c.Put(key, value)
	}
	if c.Count() != 10 {
		t.Errorf("Expected count to be 10, got %d", c.Count())
	}
	c.Clear()
	if c.Count() != 0 {
		t.Errorf("Expected count to be 0, got %d", c.Count())
	}
}
